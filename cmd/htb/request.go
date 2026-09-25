package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type clientRequestView struct {
	ID              int64     `json:"id"`
	Project         string    `json:"project"`
	Title           string    `json:"title"`
	Body            string    `json:"body"`
	Status          string    `json:"status"`
	LinkedTicketRef *string   `json:"linked_ticket_ref"`
	SubmittedBy     string    `json:"submitted_by"`
	CreatedAt       time.Time `json:"created_at"`
}

func requestCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: htb request {list|show|comments|comment|status|link}")
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("request list", flag.ContinueOnError)
		project := fs.String("project", "", "project")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("usage: htb request list [--project KEY]")
		}
		if *project == "" {
			var err error
			*project, err = currentProject()
			if err != nil {
				return err
			}
		}
		var result struct {
			Requests []clientRequestView `json:"requests"`
		}
		if err := call("GET", "/api/v1/client-requests?project="+url.QueryEscape(*project), nil, &result); err != nil {
			return err
		}
		if len(result.Requests) == 0 {
			fmt.Println("No client requests.")
			return nil
		}
		for _, item := range result.Requests {
			fmt.Printf("#%d  %-13s %s\n", item.ID, item.Status, item.Title)
		}
		return nil
	case "show", "comments", "comment", "status", "link":
		return requestByID(args)
	default:
		return errors.New("unknown request command")
	}
}

func requestByID(args []string) error {
	if len(args) < 2 {
		return errors.New("request ID is required")
	}
	id, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || id < 1 {
		return errors.New("request ID must be a positive number")
	}
	path := "/api/v1/client-requests/" + strconv.FormatInt(id, 10)
	switch args[0] {
	case "show":
		if len(args) != 2 {
			return errors.New("usage: htb request show ID")
		}
		var item clientRequestView
		if err := call("GET", path, nil, &item); err != nil {
			return err
		}
		fmt.Printf("Request #%d — %s\nProject: %s\nStatus: %s\nFrom: %s\n", item.ID, item.Title, item.Project, item.Status, item.SubmittedBy)
		if item.LinkedTicketRef != nil {
			fmt.Printf("Linked ticket: %s\n", *item.LinkedTicketRef)
		}
		fmt.Printf("\n%s\n", renderMarkdown(item.Body))
		return nil
	case "comments":
		if len(args) != 2 {
			return errors.New("usage: htb request comments ID")
		}
		var result struct {
			Comments []commentView `json:"comments"`
		}
		if err := call("GET", path+"/comments", nil, &result); err != nil {
			return err
		}
		if len(result.Comments) == 0 {
			fmt.Println("No comments.")
			return nil
		}
		for _, comment := range result.Comments {
			fmt.Printf("%s — %s\n%s\n\n", comment.CreatedAt.Format(time.RFC3339), comment.Author, renderMarkdown(comment.Body))
		}
		return nil
	case "comment":
		if len(args) != 3 {
			return errors.New("usage: htb request comment ID TEXT")
		}
		if err := call("POST", path+"/comments", map[string]string{"body": args[2]}, &struct{}{}); err != nil {
			return err
		}
		fmt.Printf("Comment added to request #%d.\n", id)
		return nil
	case "status":
		if len(args) != 3 {
			return errors.New("usage: htb request status ID received|in_progress|needs_info|done|rejected")
		}
		var item clientRequestView
		if err := call("PATCH", path, map[string]string{"status": args[2]}, &item); err != nil {
			return err
		}
		fmt.Printf("Request #%d: %s.\n", id, item.Status)
		return nil
	case "link":
		if len(args) != 3 {
			return errors.New("usage: htb request link ID TICKET-REF")
		}
		var item clientRequestView
		if err := call("PATCH", path, map[string]string{"linked_ticket_ref": args[2]}, &item); err != nil {
			return err
		}
		fmt.Printf("Request #%d linked to %s.\n", id, args[2])
		return nil
	}
	return errors.New("unknown request command")
}
