package kakao

import "encoding/json"

// LinkObject defines link targets for buttons or text.
type LinkObject struct {
	WebURL                 string `json:"web_url,omitempty"`
	MobileWebURL           string `json:"mobile_web_url,omitempty"`
	AndroidExecutionParams string `json:"android_execution_params,omitempty"`
	IOSExecutionParams     string `json:"ios_execution_params,omitempty"`
}

// ButtonObject defines a custom action button.
type ButtonObject struct {
	Title string     `json:"title"`
	Link  LinkObject `json:"link"`
}

// ContentObject represents media content in feed or list templates.
type ContentObject struct {
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	ImageURL    string     `json:"image_url,omitempty"`
	ImageWidth  int        `json:"image_width,omitempty"`
	ImageHeight int        `json:"image_height,omitempty"`
	Link        LinkObject `json:"link"`
}

// TextTemplate represents the Kakao text message template.
type TextTemplate struct {
	ObjectType  string         `json:"object_type"`
	Text        string         `json:"text"`
	Link        LinkObject     `json:"link"`
	ButtonTitle string         `json:"button_title,omitempty"`
	Buttons     []ButtonObject `json:"buttons,omitempty"`
}

// NewTextTemplate creates a simple text template.
func NewTextTemplate(text string, webURL string, buttonTitle string) *TextTemplate {
	tmpl := &TextTemplate{
		ObjectType: "text",
		Text:       text,
		Link: LinkObject{
			WebURL:       webURL,
			MobileWebURL: webURL,
		},
	}
	if buttonTitle != "" {
		tmpl.ButtonTitle = buttonTitle
	}
	return tmpl
}

// FeedTemplate represents the Kakao feed message template.
type FeedTemplate struct {
	ObjectType string         `json:"object_type"`
	Content    ContentObject  `json:"content"`
	Buttons    []ButtonObject `json:"buttons,omitempty"`
}

// NewFeedTemplate creates a feed template.
func NewFeedTemplate(title, description, imageURL, webURL, buttonTitle string) *FeedTemplate {
	feed := &FeedTemplate{
		ObjectType: "feed",
		Content: ContentObject{
			Title:       title,
			Description: description,
			ImageURL:    imageURL,
			Link: LinkObject{
				WebURL:       webURL,
				MobileWebURL: webURL,
			},
		},
	}
	if buttonTitle != "" {
		feed.Buttons = []ButtonObject{
			{
				Title: buttonTitle,
				Link: LinkObject{
					WebURL:       webURL,
					MobileWebURL: webURL,
				},
			},
		}
	}
	return feed
}

// ListTemplate represents the Kakao list message template.
type ListTemplate struct {
	ObjectType  string          `json:"object_type"`
	HeaderTitle string          `json:"header_title"`
	HeaderLink  LinkObject      `json:"header_link"`
	Contents    []ContentObject `json:"contents"`
	Buttons     []ButtonObject  `json:"buttons,omitempty"`
}

// ToJSON marshals any template into a JSON string.
func ToJSON(v interface{}) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
