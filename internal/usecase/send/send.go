package send

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dh-kam/kakao-bot/pkg/kakao"
)

// SendMeRequest holds options for sending a message to oneself.
type SendMeRequest struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	Text         string
	WebURL       string
	ButtonTitle  string
	Title        string
	Description  string
	ImageURL     string
	TemplateID   int64
	TemplateArgs string
	In           io.Reader
	Out          io.Writer
}

// SendMeUseCase handles sending messages to oneself.
type SendMeUseCase struct{}

func NewSendMeUseCase() *SendMeUseCase {
	return &SendMeUseCase{}
}

func (uc *SendMeUseCase) Execute(ctx context.Context, req SendMeRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}
	in := req.In
	if in == nil {
		in = os.Stdin
	}

	client := buildClient(req.ClientID, req.ClientSecret, req.RedirectURI, req.TokenPath)

	// Check if custom template ID is provided
	if req.TemplateID > 0 {
		var args map[string]string
		if req.TemplateArgs != "" {
			if err := json.Unmarshal([]byte(req.TemplateArgs), &args); err != nil {
				return fmt.Errorf("invalid template args JSON: %w", err)
			}
		}
		if err := client.SendMeCustom(ctx, req.TemplateID, args); err != nil {
			return fmt.Errorf("send custom message: %w", err)
		}
		fmt.Fprintf(out, "Custom template message (ID: %d) sent successfully to yourself.\n", req.TemplateID)
		return nil
	}

	// Check if feed template fields are provided
	if req.Title != "" || req.ImageURL != "" {
		feed := kakao.NewFeedTemplate(req.Title, req.Description, req.ImageURL, req.WebURL, req.ButtonTitle)
		if err := client.SendMeFeed(ctx, feed); err != nil {
			return fmt.Errorf("send feed message: %w", err)
		}
		fmt.Fprintln(out, "Feed message sent successfully to yourself.")
		return nil
	}

	// Otherwise, handle text message
	text := req.Text
	if text == "" || text == "-" {
		// Read from stdin if text is empty or "-"
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 || text == "-" {
			bytes, err := io.ReadAll(in)
			if err != nil {
				return fmt.Errorf("read text from stdin: %w", err)
			}
			text = strings.TrimSpace(string(bytes))
		}
	}

	if text == "" {
		return fmt.Errorf("message text cannot be empty (pass as argument, --text flag, or pipe via stdin)")
	}

	if err := client.SendMeText(ctx, text, req.WebURL, req.ButtonTitle); err != nil {
		return fmt.Errorf("send text message: %w", err)
	}

	fmt.Fprintln(out, "Message sent successfully to yourself.")
	return nil
}

// SendFriendRequest holds options for sending a message to Kakao friends.
type SendFriendRequest struct {
	ClientID      string
	ClientSecret  string
	RedirectURI   string
	TokenPath     string
	ReceiverUUIDs []string
	Text          string
	WebURL        string
	ButtonTitle   string
	Title         string
	Description   string
	ImageURL      string
	TemplateID    int64
	TemplateArgs  string
	In            io.Reader
	Out           io.Writer
}

// SendFriendUseCase handles sending messages to friends.
type SendFriendUseCase struct{}

func NewSendFriendUseCase() *SendFriendUseCase {
	return &SendFriendUseCase{}
}

func (uc *SendFriendUseCase) Execute(ctx context.Context, req SendFriendRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}
	in := req.In
	if in == nil {
		in = os.Stdin
	}

	if len(req.ReceiverUUIDs) == 0 {
		return fmt.Errorf("at least one receiver UUID is required (--receiver-uuids)")
	}

	client := buildClient(req.ClientID, req.ClientSecret, req.RedirectURI, req.TokenPath)

	if req.TemplateID > 0 {
		var args map[string]string
		if req.TemplateArgs != "" {
			if err := json.Unmarshal([]byte(req.TemplateArgs), &args); err != nil {
				return fmt.Errorf("invalid template args JSON: %w", err)
			}
		}
		res, err := client.SendFriendsCustom(ctx, req.ReceiverUUIDs, req.TemplateID, args)
		if err != nil {
			return fmt.Errorf("send custom message to friends: %w", err)
		}
		printFriendResult(out, res)
		return nil
	}

	if req.Title != "" || req.ImageURL != "" {
		feed := kakao.NewFeedTemplate(req.Title, req.Description, req.ImageURL, req.WebURL, req.ButtonTitle)
		res, err := client.SendFriendsTemplate(ctx, req.ReceiverUUIDs, feed)
		if err != nil {
			return fmt.Errorf("send feed message to friends: %w", err)
		}
		printFriendResult(out, res)
		return nil
	}

	text := req.Text
	if text == "" || text == "-" {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 || text == "-" {
			bytes, err := io.ReadAll(in)
			if err != nil {
				return fmt.Errorf("read text from stdin: %w", err)
			}
			text = strings.TrimSpace(string(bytes))
		}
	}

	if text == "" {
		return fmt.Errorf("message text cannot be empty")
	}

	res, err := client.SendFriendsText(ctx, req.ReceiverUUIDs, text, req.WebURL, req.ButtonTitle)
	if err != nil {
		return fmt.Errorf("send text message to friends: %w", err)
	}

	printFriendResult(out, res)
	return nil
}

func printFriendResult(out io.Writer, res *kakao.MessageResult) {
	if len(res.SuccessfulReceiverUUIDs) > 0 {
		fmt.Fprintf(out, "Successfully delivered to %d recipient(s): %s\n",
			len(res.SuccessfulReceiverUUIDs), strings.Join(res.SuccessfulReceiverUUIDs, ", "))
	}
	if len(res.FailureInfo) > 0 {
		fmt.Fprintf(out, "Failed for %d group(s):\n", len(res.FailureInfo))
		for _, f := range res.FailureInfo {
			fmt.Fprintf(out, "  - [Code %d] %s: %s\n", f.Code, f.Msg, strings.Join(f.ReceiverUUIDs, ", "))
		}
	}
}

func buildClient(clientID, clientSecret, redirectURI, tokenPath string) *kakao.Client {
	oauthClient := kakao.NewOAuthClient(kakao.OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
	})
	tokenStore := kakao.NewFileTokenStore(tokenPath)
	return kakao.NewClient(kakao.ClientConfig{
		OAuthClient: oauthClient,
		TokenStore:  tokenStore,
	})
}
