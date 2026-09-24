package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kazerlelutin/htb/internal/domain"
)

func clientStoryPath(ref string) (string, error) {
	if _, _, err := domain.ParseReference(ref); err != nil {
		return "", errors.New("a valid user-story reference is required")
	}
	return "/api/v1/client-stories/" + strings.ToUpper(ref), nil
}

func ticketPublication(args []string, published bool) error {
	command := "publish"
	method := "PUT"
	if !published {
		command, method = "unpublish", "DELETE"
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: htb ticket %s STORY-REF", command)
	}
	path, err := clientStoryPath(args[0])
	if err != nil {
		return err
	}
	var response struct {
		Ref string `json:"ref"`
	}
	if err := call(method, path+"/publication", nil, &response); err != nil {
		return err
	}
	if published {
		fmt.Printf("User story %s published in the client portal.\n", response.Ref)
	} else {
		fmt.Printf("User story %s hidden from the client portal.\n", response.Ref)
	}
	return nil
}

func ticketClientComments(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: htb ticket client-comments STORY-REF")
	}
	path, err := clientStoryPath(args[0])
	if err != nil {
		return err
	}
	var response struct {
		Comments []commentView `json:"comments"`
	}
	if err := call("GET", path+"/comments", nil, &response); err != nil {
		return err
	}
	if len(response.Comments) == 0 {
		fmt.Println("No client comments.")
		return nil
	}
	for _, comment := range response.Comments {
		fmt.Printf("%s — %s\n%s\n\n", comment.CreatedAt.Format(time.RFC3339), comment.Author, renderMarkdown(comment.Body))
	}
	return nil
}

func ticketClientComment(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: htb ticket client-comment STORY-REF TEXT")
	}
	path, err := clientStoryPath(args[0])
	if err != nil {
		return err
	}
	if err := call("POST", path+"/comments", map[string]string{"body": args[1]}, &struct{}{}); err != nil {
		return err
	}
	fmt.Printf("Client-visible comment added to %s.\n", strings.ToUpper(args[0]))
	return nil
}
