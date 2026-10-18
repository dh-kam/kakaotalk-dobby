package friends

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/samber/lo"
	"golang.org/x/term"
)

// ServiceAccountInfo encapsulates active service account identity details.
type ServiceAccountInfo struct {
	ID          int64
	Nickname    string
	Email       string
	ConnectedAt string
	Scope       string
}

// FriendItemView encapsulates display data for a friend item.
type FriendItemView struct {
	Index        int
	UUID         string
	Nickname     string
	Email        string
	MappedName   string
	IsAdmin      bool
	IsRegistered bool
	Messageable  bool
	Favorite     bool
}

// FriendsViewData contains all data required to render the friends and service account view.
type FriendsViewData struct {
	ServiceAccount *ServiceAccountInfo
	Friends        []FriendItemView
	TotalCount     int
}

// FriendsPresenter renders FriendsViewData to markdown and terminal glamour styles.
type FriendsPresenter struct{}

// NewFriendsPresenter creates a new FriendsPresenter instance.
func NewFriendsPresenter() *FriendsPresenter {
	return &FriendsPresenter{}
}

// BuildMarkdown generates a structured GitHub-flavored Markdown representation.
func (p *FriendsPresenter) BuildMarkdown(data FriendsViewData) string {
	var sb strings.Builder

	sb.WriteString("# 🤖 KakaoTalk Service Account & Friends\n\n")

	// Service Account section (compact bullet list to avoid wide 2-column table wrapping)
	if data.ServiceAccount != nil && (data.ServiceAccount.ID != 0 || data.ServiceAccount.Scope != "") {
		sb.WriteString("### 👤 Active Service Account\n\n")
		if data.ServiceAccount.ID != 0 {
			sb.WriteString(fmt.Sprintf("- **User ID**: `%d`\n", data.ServiceAccount.ID))
		}
		if data.ServiceAccount.Nickname != "" {
			sb.WriteString(fmt.Sprintf("- **Nickname**: %s\n", data.ServiceAccount.Nickname))
		}
		if data.ServiceAccount.Email != "" {
			sb.WriteString(fmt.Sprintf("- **Email**: `%s`\n", data.ServiceAccount.Email))
		}
		if data.ServiceAccount.ConnectedAt != "" {
			formattedTime := data.ServiceAccount.ConnectedAt
			if t, err := time.Parse(time.RFC3339, data.ServiceAccount.ConnectedAt); err == nil {
				formattedTime = t.Local().Format("2006-01-02 15:04:05 MST")
			}
			sb.WriteString(fmt.Sprintf("- **Connected At**: %s\n", formattedTime))
		}
		if data.ServiceAccount.Scope != "" {
			scopes := lo.Map(strings.Fields(data.ServiceAccount.Scope), func(s string, _ int) string {
				return fmt.Sprintf("`%s`", s)
			})
			sb.WriteString(fmt.Sprintf("- **Token Scopes**: %s\n", strings.Join(scopes, ", ")))
		}
		sb.WriteString("\n---\n\n")
	}

	// Friends section
	if len(data.Friends) == 0 {
		sb.WriteString("### 👥 Authorized Friends (0)\n\n")
		sb.WriteString("> ⚠️ **No friends found.** Friends must authenticate and grant messaging consent to this application.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("### 👥 Authorized Friends (%d / Total %d)\n\n", len(data.Friends), data.TotalCount))
	sb.WriteString("| # | Nickname | Email | Status | Receiver UUID |\n")
	sb.WriteString("| :-: | :--- | :--- | :-: | :--- |\n")

	unregisteredCount := 0
	lo.ForEach(data.Friends, func(f FriendItemView, _ int) {
		msgStatus := "❌ No"
		if f.Messageable {
			msgStatus = "✅ Yes"
		}

		nickname := f.Nickname
		if f.MappedName != "" && f.MappedName != f.Nickname {
			nickname = fmt.Sprintf("%s (%s)", f.Nickname, f.MappedName)
		}
		if f.IsAdmin {
			nickname = fmt.Sprintf("%s 👑", nickname)
		} else if f.Favorite {
			nickname = fmt.Sprintf("%s ⭐", nickname)
		}

		email := f.Email
		if email == "" {
			if !f.IsRegistered {
				email = "⚠️ 미등록"
				unregisteredCount++
			} else {
				email = "-"
			}
		} else {
			email = fmt.Sprintf("`%s`", email)
		}

		sb.WriteString(fmt.Sprintf("| %d | **%s** | %s | %s | `%s` |\n",
			f.Index,
			nickname,
			email,
			msgStatus,
			f.UUID,
		))
	})

	if unregisteredCount > 0 {
		sb.WriteString(fmt.Sprintf("\n> 🚨 **보안 주의**: 등록되지 않은 미승인 친구(%d명)가 감지되었습니다. 화이트리스트(`friends_map.json`)를 점검하세요.\n", unregisteredCount))
	}

	return sb.String()
}

// getTerminalWidth dynamically detects terminal column width from out, falling back to 110.
func getTerminalWidth(out io.Writer) int {
	type fileDescriptor interface {
		Fd() uintptr
	}

	if f, ok := out.(fileDescriptor); ok {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			// Clamp to safe word wrap width:
			// Subtract 4 columns to avoid edge overflows on narrow or split panes,
			// and bound between 80 and 130 for readable table proportions.
			return lo.Clamp(w-4, 80, 130)
		}
	}
	return 110
}

// Render writes glamour-styled ANSI output (or raw markdown if requested) to out.
func (p *FriendsPresenter) Render(out io.Writer, data FriendsViewData, rawMarkdown bool) error {
	md := p.BuildMarkdown(data)
	if rawMarkdown {
		_, err := fmt.Fprint(out, md)
		return err
	}

	width := getTerminalWidth(out)
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		_, fallbackErr := fmt.Fprint(out, md)
		return fallbackErr
	}

	rendered, err := renderer.Render(md)
	if err != nil {
		_, fallbackErr := fmt.Fprint(out, md)
		return fallbackErr
	}

	_, err = fmt.Fprint(out, rendered)
	return err
}
