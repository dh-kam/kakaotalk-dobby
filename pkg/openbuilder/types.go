package openbuilder

// SkillPayload represents the incoming HTTP POST request body from Kakao i OpenBuilder.
type SkillPayload struct {
	UserRequest UserRequest `json:"userRequest"`
	Bot         Bot         `json:"bot"`
	Action      Action      `json:"action"`
	Contexts    []Context   `json:"contexts,omitempty"`
}

// UserRequest contains user-specific request details and user message.
type UserRequest struct {
	Timezone  string                 `json:"timezone"`
	Params    map[string]interface{} `json:"params,omitempty"`
	Block     Block                  `json:"block"`
	Utterance string                 `json:"utterance"`
	Lang      string                 `json:"lang,omitempty"`
	User      ChatUser               `json:"user"`
}

// Block represents the matched scenario block in OpenBuilder.
type Block struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ChatUser identifies the end user talking to the bot.
type ChatUser struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Bot contains bot information.
type Bot struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Action contains matched action and extracted parameters.
type Action struct {
	Name         string                 `json:"name"`
	ID           string                 `json:"id"`
	Params       map[string]string      `json:"params,omitempty"`
	DetailParams map[string]interface{} `json:"detailParams,omitempty"`
	ClientExtra  map[string]interface{} `json:"clientExtra,omitempty"`
}

// Context represents context state passed from OpenBuilder.
type Context struct {
	Name     string                 `json:"name"`
	LifeSpan int                    `json:"lifeSpan"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

// SkillResponse is the response format expected by Kakao i OpenBuilder (v2.0).
type SkillResponse struct {
	Version     string        `json:"version"`
	Template    SkillTemplate `json:"template"`
	Context     *SkillContext `json:"context,omitempty"`
	Data        interface{}   `json:"data,omitempty"`
	UseCallback bool          `json:"useCallback,omitempty"`
}

// SkillTemplate contains UI output components.
type SkillTemplate struct {
	Outputs      []Output     `json:"outputs"`
	QuickReplies []QuickReply `json:"quickReplies,omitempty"`
}

// Output is a single UI message bubble/card container.
type Output struct {
	SimpleText   *SimpleText   `json:"simpleText,omitempty"`
	SimpleImage  *SimpleImage  `json:"simpleImage,omitempty"`
	TextCard     *TextCard     `json:"textCard,omitempty"`
	ItemCard     *ItemCard     `json:"itemCard,omitempty"`
	BasicCard    *BasicCard    `json:"basicCard,omitempty"`
	CommerceCard *CommerceCard `json:"commerceCard,omitempty"`
	ListCard     *ListCard     `json:"listCard,omitempty"`
	Carousel     *Carousel     `json:"carousel,omitempty"`
}

// SimpleText represents simple text bubble.
type SimpleText struct {
	Text string `json:"text"`
}

// SimpleImage represents simple image bubble.
type SimpleImage struct {
	ImageURL string `json:"imageUrl"`
	AltText  string `json:"altText"`
}

// TextCard represents simple text card.
type TextCard struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description"`
	Buttons     []CardButton `json:"buttons,omitempty"`
}

// ItemCard represents KakaoTalk rich key-value item card.
type ItemCard struct {
	ImageTitle        *ItemCardImageTitle `json:"imageTitle,omitempty"`
	Title             string              `json:"title,omitempty"`
	Description       string              `json:"description,omitempty"`
	ItemList          []ItemCardItem      `json:"itemList"`
	ItemListAlignment string              `json:"itemListAlignment,omitempty"` // "left" or "right"
	ItemListSummary   *ItemCardSummary    `json:"itemListSummary,omitempty"`
	Buttons           []CardButton        `json:"buttons,omitempty"`
	ButtonLayout      string              `json:"buttonLayout,omitempty"`
}

// ItemCardImageTitle represents header with title and description.
type ItemCardImageTitle struct {
	ImageURL    string `json:"imageUrl,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// ItemCardItem is a key-value row inside ItemCard.
type ItemCardItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// ItemCardSummary is the summary footer row inside ItemCard.
type ItemCardSummary struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// BasicCard represents card with thumbnail, title, description, and buttons.
type BasicCard struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	Thumbnail   *Thumbnail   `json:"thumbnail,omitempty"`
	Buttons     []CardButton `json:"buttons,omitempty"`
}

// CommerceCard represents product card with price and discount.
type CommerceCard struct {
	Title           string       `json:"title"`
	Description     string       `json:"description,omitempty"`
	Price           int          `json:"price"`
	Currency        string       `json:"currency,omitempty"`
	Discount        int          `json:"discount,omitempty"`
	DiscountRate    int          `json:"discountRate,omitempty"`
	DiscountedPrice int          `json:"discountedPrice,omitempty"`
	Thumbnails      []Thumbnail  `json:"thumbnails"`
	Buttons         []CardButton `json:"buttons"`
}

// ListCard represents structured list items.
type ListCard struct {
	Header  ListHeader   `json:"header"`
	Items   []ListItem   `json:"items"`
	Buttons []CardButton `json:"buttons,omitempty"`
}

// ListHeader is the header of a ListCard.
type ListHeader struct {
	Title string `json:"title"`
}

// ListItem is an item entry inside ListCard.
type ListItem struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	ImageURL    string `json:"imageUrl,omitempty"`
	Link        *Link  `json:"link,omitempty"`
}

// Carousel represents horizontal scrollable card list.
type Carousel struct {
	Type  string      `json:"type"` // "basicCard" or "commerceCard" or "itemCard"
	Items interface{} `json:"items"`
}

// Thumbnail represents image in cards.
type Thumbnail struct {
	ImageURL   string `json:"imageUrl"`
	Link       *Link  `json:"link,omitempty"`
	FixedRatio bool   `json:"fixedRatio,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
}

// CardButton represents action button in cards.
type CardButton struct {
	Label       string                 `json:"label"`
	Action      string                 `json:"action"` // "webLink", "message", "phone", "block", "share"
	WebLinkURL  string                 `json:"webLinkUrl,omitempty"`
	MessageText string                 `json:"messageText,omitempty"`
	PhoneNumber string                 `json:"phoneNumber,omitempty"`
	BlockID     string                 `json:"blockId,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// QuickReply represents quick suggestion buttons below message.
type QuickReply struct {
	Label       string                 `json:"label"`
	Action      string                 `json:"action"` // "message" or "block"
	MessageText string                 `json:"messageText,omitempty"`
	BlockID     string                 `json:"blockId,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// Link represents link target.
type Link struct {
	Web     string `json:"web,omitempty"`
	Mobile  string `json:"mobile,omitempty"`
	Android string `json:"android,omitempty"`
	IOS     string `json:"ios,omitempty"`
}

// SkillContext contains context values to be preserved.
type SkillContext struct {
	Values []ContextValue `json:"values"`
}

// ContextValue represents key-value context.
type ContextValue struct {
	Name     string                 `json:"name"`
	LifeSpan int                    `json:"lifeSpan"`
	Params   map[string]interface{} `json:"params,omitempty"`
}
