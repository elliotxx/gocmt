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
	"strings"

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

// AddComments adds comments to the specified Go source file based on the JSON structure.
func AddComments(goCode string, commentsJSON string) (string, error) {
	// Unmarshal the JSON string into a slice of Comment structs.
	var comments CommentJSON
	err := json.Unmarshal([]byte(commentsJSON), &comments)
	if err != nil {
		return "", err
	}

	// Split the file contents into lines.
	lines := strings.Split(goCode, "\n")

	// Process each comment and add it to the appropriate line.
	for _, comment := range comments.Comments {
		// Find the line number to insert the comment.
		position := comment.Position
		lineNumber := 0
		for _, line := range lines {
			if strings.Contains(line, position) {
				break
			}
			lineNumber++
		}

		// If the position is found, insert the comment above the line.
		if lineNumber < len(lines) {
			lines[lineNumber] = "// " + comment.Comment + "\n" + lines[lineNumber]
		}
	}

	// Join the lines back into a single string.
	newContents := strings.Join(lines, "\n")

	return newContents, nil
}

func main() {
	// Setting up logger
	logFile, err := os.OpenFile("logfile.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Failed to open log file:", err)
		return
	}
	defer logFile.Close()

	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

	// Parse command line arguments
	fileName := flag.String("file", "", "File containing Go code")
	flag.Parse()

	if *fileName == "" {
		log.Println("Please provide a file containing Go code using -file flag.")
		return
	}

	// Read Go code from file
	goCode, err := os.ReadFile(*fileName)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return
	}

	// Process Go code
	processedCode, err := processGoCode(string(goCode))
	if err != nil {
		log.Printf("Error processing Go code: %v", err)
		return
	}
	log.Println(processedCode)

	token := os.Getenv("MOONSHOT_API_KEY")
	if token == "" {
		log.Println("The environment variable MOONSHOT_API_KEY is not set.")
		return
	}

	client := NewMoonShotClient(token)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       "moonshot-v1-8k",
			Temperature: 0.3,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(`### Role ###
You are a Go language expert with a solid foundation in Go and high standards for code comments. Additionally, your English is excellent, enabling you to write professional English comments.

### Requirements ###
- Add appropriate comments above each structure, method, function, and other key code.
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
	log.Println(commentsJSON)

	// Add the comments to the file.
	result, err := AddComments(string(goCode), commentsJSON)
	if err != nil {
		log.Printf("Error adding comments to the file: %v", err)
		os.Exit(1)
	}

	formatResult, err := formatGoCode(result)
	if err != nil {
		log.Printf("Error format go code: %v", err)
		os.Exit(1)
	}
	log.Println(formatResult)

	err = os.WriteFile(*fileName, []byte(formatResult), 0644)
	if err != nil {
		log.Printf("Failed to write Go code to file: %v", err)
		os.Exit(1)
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
