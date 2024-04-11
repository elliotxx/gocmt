package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	openai "github.com/sashabaranov/go-openai"
)

// NewMoonShotClient creates a new MoonShot API client.
func NewMoonShotClient(authToken string) *openai.Client {
	config := openai.DefaultConfig(authToken)
	config.BaseURL = "https://api.moonshot.cn/v1"
	return openai.NewClientWithConfig(config)
}

type CommentJSON struct {
	Comments []Comment `json:"comments"`
}

// Comment represents the structure of a comment in the JSON.
type Comment struct {
	Position string `json:"position"`
	Comment  string `json:"comment"`
}

func main() {
	// Setting up logger
	logFile, err := os.OpenFile("logfile.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Failed to open log file:", err)
		return
	}
	defer logFile.Close()

	log.SetOutput(io.MultiWriter(logFile))

	// Parse command line arguments
	concurrency := flag.Int("n", 1, "number of concurrent executions")
	fileOrDir := flag.String("f", "", "File or directory containing Go code")
	flag.Parse()

	if *fileOrDir == "" {
		log.Println("Please provide a file or directory containing Go code using -f flag.")
		return
	}

	var goFiles []string

	// Check if the specified path is a directory or a file
	fileInfo, err := os.Stat(*fileOrDir)
	if err != nil {
		log.Printf("Error accessing file or directory: %v", err)
		return
	}

	if fileInfo.IsDir() {
		// If it's a directory, recursively find all Go files
		err := filepath.Walk(*fileOrDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go") {
				goFiles = append(goFiles, path)
			}
			return nil
		})
		if err != nil {
			log.Printf("Error walking directory: %v", err)
			return
		}
	} else {
		// If it's a file, add it directly
		goFiles = append(goFiles, *fileOrDir)
	}

	// Create MoonShot API client
	token := os.Getenv("MOONSHOT_API_KEY")
	if token == "" {
		fmt.Println("The environment variable MOONSHOT_API_KEY is not set.")
		os.Exit(1)
	}
	client := NewMoonShotClient(token)

	// Process each Go file
	total := len(goFiles)
	var wg sync.WaitGroup
	sem := make(chan struct{}, *concurrency)
	done := make(chan bool)
	progress := make(chan int)
	var completed int32

	go func() {
		for range progress {
			atomic.AddInt32(&completed, 1)
			var percent float64
			if int(completed) >= total {
				percent = 100.0
			} else if int(completed) == 0 {
				percent = 0.0
			} else {
				percent = float64(completed) / float64(total) * 100
			}
			fmt.Printf("\rProgress: %d/%d, %.2f%%\n", completed, total, percent)
			if int(completed) >= total {
				fmt.Println("\nAll files processed.")
				done <- true
				break
			}
		}
	}()

	for i, file := range goFiles {
		wg.Add(1)
		sem <- struct{}{}

		go func(i int, file string) {
			var (
				err           error
				goCodeByte    []byte
				processedCode string
				resp          openai.ChatCompletionResponse
				result        string
				formatResult  string
			)
			defer func() {
				if err != nil {
					fmt.Printf("Error: %v, File: %s\n", err, file)
				}
				<-sem
				wg.Done()
				progress <- i
			}()
			log.Printf("Processing file: %s", file)
			fmt.Printf("» Processing %s...\n", file)

			// Read Go code from file
			goCodeByte, err = os.ReadFile(file)
			if err != nil {
				log.Printf("Error reading file: %v", err)
				return
			}
			goCode := string(goCodeByte)

			// Format Go code
			goCode, err = formatGoCode(goCode)
			if err != nil {
				log.Printf("Error format go code: %v", err)
				return
			}

			// Process Go code
			processedCode, err = processGoCode(goCode)
			if err != nil {
				log.Printf("Error processing Go code: %v", err)
				return
			}

			// Perform API request and get comments
			resp, err = client.CreateChatCompletion(
				context.Background(),
				openai.ChatCompletionRequest{
					Model:       "moonshot-v1-8k",
					Temperature: 0.3,
					MaxTokens:   4096,
					Messages: []openai.ChatCompletionMessage{
						{
							Role: openai.ChatMessageRoleUser,
							Content: fmt.Sprintf(`### Role ###
You are a Go language expert with a solid foundation in Go and high standards for code comments. Additionally, your English is excellent, enabling you to write professional English comments.
### Requirements ###
- Add meaningful and technical comments above each structure, method, function, and other key code.
- Mark the code position and supplementary annotations in a structured manner, and output all the comments that need to be supplemented in JSON format
- The return result is plain text, and three backticks are not needed.
### Output Format Example ###
{
    "comments": [
        {
            "position": "type MockManagerInterface interface {",
            "comment": "MockManagerInterface defines the interface for mock manager."
        },
        {
            "position": "type mockManager struct {",
            "comment": "mockManager is the implementation that mock manager."
        }
    ]
}
### Target Code ###
%s`, processedCode),
						},
					},
				},
			)
			if err != nil {
				log.Printf("ChatCompletion error: %v", err)
				return
			}
			commentsJSON := resp.Choices[0].Message.Content

			// Add the comments to the file.
			result, err = addComments(goCode, commentsJSON)
			if err != nil {
				log.Printf("Error adding comments to the file: %v", err)
				return
			}

			fmt.Printf("✔ Processed file %s\n", file)

			formatResult, err = formatGoCode(result)
			if err != nil {
				log.Printf("Error format go code: %v", err)
				return
			}

			err = os.WriteFile(file, []byte(formatResult), 0644)
			if err != nil {
				log.Printf("Failed to write Go code to file: %v", err)
				return
			}
		}(i, file)
	}

	go func() {
		wg.Wait()
		close(progress)
	}()

	<-done
}

// addComments adds comments to the specified Go source file based on the JSON structure.
func addComments(goCode string, commentsJSON string) (string, error) {
	// Unmarshal the JSON string into a slice of Comment structs.
	var comments CommentJSON
	if err := json.Unmarshal([]byte(commentsJSON), &comments); err != nil {
		return "", err
	}
	// Parse Go code into an AST (Abstract Syntax Tree).
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", goCode, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parsing Go code: %v", err)
	}

	// Create an ast.CommentMap from the ast.File's comments.
	// This helps keeping the association between comments
	// and AST nodes.
	cmap := ast.NewCommentMap(fset, node, node.Comments)

	// Traverse the AST to find comment positions and add comments.
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			code := goCode[fset.Position(x.Pos()).Offset:fset.Position(x.End()).Offset]
			addFunctionComments(cmap, code, x, comments.Comments)
			// case *ast.TypeSpec:
			// 	code := goCode[fset.Position(x.Pos()).Offset:fset.Position(x.End()).Offset]
			// 	addTypeComments(cmap, code, x, comments.Comments)
			// case *ast.GenDecl:
			// 	code := goCode[fset.Position(x.Pos()).Offset:fset.Position(x.End()).Offset]
			// 	addGeneralComments(cmap, code, x, comments.Comments)
		}
		return true
	})

	// Use the comment map to filter comments that don't belong anymore
	// (the comments associated with the variable declaration), and create
	// the new comments list.
	node.Comments = cmap.Filter(node).Comments()

	// Write the modified AST back to a string.
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return "", fmt.Errorf("formatting Go code: %v", err)
	}
	return buf.String(), nil
}

// addFunctionComments adds comments to function declarations based on position.
func addFunctionComments(cmap ast.CommentMap, code string, decl *ast.FuncDecl, comments []Comment) {
	for _, comment := range comments {
		if strings.Contains(code, comment.Position) && decl.Doc == nil {
			cmap[decl] = []*ast.CommentGroup{
				{
					List: []*ast.Comment{
						{
							Slash: decl.Pos() - 1,
							Text:  "// " + comment.Comment,
						},
					},
				},
			}
			break
		}
	}
}

// addTypeComments adds comments to type declarations based on position.
func addTypeComments(cmap ast.CommentMap, code string, decl *ast.TypeSpec, comments []Comment) {
	for _, comment := range comments {
		if strings.Contains(code, comment.Position) && decl.Doc == nil {
			cmap[decl] = []*ast.CommentGroup{
				{
					List: []*ast.Comment{
						{
							Slash: decl.Name.NamePos - 6,
							Text:  "// " + comment.Comment,
						},
					},
				},
			}
			break
		}
	}
}

// addGeneralComments adds comments to general declarations (e.g., variables) based on position.
func addGeneralComments(cmap ast.CommentMap, code string, decl *ast.GenDecl, comments []Comment) {
	for _, spec := range decl.Specs {
		switch x := spec.(type) {
		case *ast.TypeSpec:
			addTypeComments(cmap, code, x, comments)
		}
	}
}

func processGoCode(goCode string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", goCode, 0)
	if err != nil {
		return "", fmt.Errorf("parsing Go code: %w", err)
	}

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Body != nil {
				replaceFuncBody(x)
			}
		}
		return true
	})

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return "", fmt.Errorf("formatting Go code: %w", err)
	}
	return buf.String(), nil
}

func formatGoCode(goCode string) (string, error) {
	// Format the provided Go code
	formatted, err := format.Source([]byte(goCode))
	if err != nil {
		return "", fmt.Errorf("failed to format Go code: %v", err)
	}
	return string(formatted), nil
}

func replaceFuncBody(decl *ast.FuncDecl) {
	// Replace function body with empty string.
	decl.Body = &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ExprStmt{
				X: &ast.BasicLit{
					Kind:  token.STRING,
					Value: ``,
				},
			},
		},
	}
}
