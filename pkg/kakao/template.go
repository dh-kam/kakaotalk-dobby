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

// ContentObject represents media content in feed, list, or location templates.
type ContentObject struct {
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	ImageURL    string     `json:"image_url,omitempty"`
	ImageWidth  int        `json:"image_width,omitempty"`
	ImageHeight int        `json:"image_height,omitempty"`
	Link        LinkObject `json:"link"`
}

// SocialObject contains social reaction counts.
type SocialObject struct {
	LikeCount        int `json:"like_count,omitempty"`
	CommentCount     int `json:"comment_count,omitempty"`
	SharedCount      int `json:"shared_count,omitempty"`
	ViewCount        int `json:"view_count,omitempty"`
	SubscriberCount  int `json:"subscriber_count,omitempty"`
}

// CommerceObject contains price and product discount details.
type CommerceObject struct {
	RegularPrice       int    `json:"regular_price"`
	DiscountPrice      int    `json:"discount_price,omitempty"`
	DiscountRate       int    `json:"discount_rate,omitempty"`
	FixedDiscountPrice int    `json:"fixed_discount_price,omitempty"`
	ProductName        string `json:"product_name,omitempty"`
	CurrencyUnit       string `json:"currency_unit,omitempty"`
	CurrencyUnitPos    int    `json:"currency_unit_position,omitempty"`
}

// ItemInfo represents an item in an ItemContentObject.
type ItemInfo struct {
	Item   string `json:"item"`
	ItemOp string `json:"item_op"`
}

// ItemContentObject represents supplementary itemized content in feed or list.
type ItemContentObject struct {
	ProfileText         string     `json:"profile_text,omitempty"`
	ProfileImageURL     string     `json:"profile_image_url,omitempty"`
	TitleImageURL       string     `json:"title_image_url,omitempty"`
	TitleImageText      string     `json:"title_image_text,omitempty"`
	TitleImageCategory  string     `json:"title_image_category,omitempty"`
	Items               []ItemInfo `json:"items,omitempty"`
	Sum                 string     `json:"sum,omitempty"`
	SumOp               string     `json:"sum_op,omitempty"`
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
	ObjectType  string             `json:"object_type"`
	Content     ContentObject      `json:"content"`
	ItemContent *ItemContentObject `json:"item_content,omitempty"`
	Social      *SocialObject      `json:"social,omitempty"`
	Buttons     []ButtonObject     `json:"buttons,omitempty"`
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

// CommerceTemplate represents the Kakao commerce message template.
type CommerceTemplate struct {
	ObjectType string         `json:"object_type"`
	Content    ContentObject  `json:"content"`
	Commerce   CommerceObject `json:"commerce"`
	Buttons    []ButtonObject `json:"buttons,omitempty"`
}

// LocationTemplate represents the Kakao location map message template.
type LocationTemplate struct {
	ObjectType   string         `json:"object_type"`
	Content      ContentObject  `json:"content"`
	Address      string         `json:"address"`
	AddressTitle string         `json:"address_title,omitempty"`
	Buttons      []ButtonObject `json:"buttons,omitempty"`
}

// ToJSON marshals any template into a JSON string.
func ToJSON(v interface{}) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
