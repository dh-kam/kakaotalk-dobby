package openbuilder

// NewResponse creates an empty SkillResponse with version 2.0.
func NewResponse() *SkillResponse {
	return &SkillResponse{
		Version: "2.0",
		Template: SkillTemplate{
			Outputs:      make([]Output, 0),
			QuickReplies: make([]QuickReply, 0),
		},
	}
}

// NewSimpleTextResponse creates a quick simple text reply.
func NewSimpleTextResponse(text string) *SkillResponse {
	resp := NewResponse()
	resp.AddSimpleText(text)
	return resp
}

// NewBasicCardResponse creates a basic card reply.
func NewBasicCardResponse(title, desc, imageURL string, buttons ...CardButton) *SkillResponse {
	resp := NewResponse()
	card := &BasicCard{
		Title:       title,
		Description: desc,
		Buttons:     buttons,
	}
	if imageURL != "" {
		card.Thumbnail = &Thumbnail{
			ImageURL: imageURL,
		}
	}
	resp.AddBasicCard(card)
	return resp
}

// AddSimpleText adds a simple text bubble to the response.
func (r *SkillResponse) AddSimpleText(text string) *SkillResponse {
	r.Template.Outputs = append(r.Template.Outputs, Output{
		SimpleText: &SimpleText{Text: text},
	})
	return r
}

// AddSimpleImage adds a simple image bubble to the response.
func (r *SkillResponse) AddSimpleImage(imageURL, altText string) *SkillResponse {
	r.Template.Outputs = append(r.Template.Outputs, Output{
		SimpleImage: &SimpleImage{ImageURL: imageURL, AltText: altText},
	})
	return r
}

// AddBasicCard adds a basic card to the response.
func (r *SkillResponse) AddBasicCard(card *BasicCard) *SkillResponse {
	r.Template.Outputs = append(r.Template.Outputs, Output{
		BasicCard: card,
	})
	return r
}

// AddQuickReply adds a quick reply button suggestion.
func (r *SkillResponse) AddQuickReply(label, messageText string) *SkillResponse {
	r.Template.QuickReplies = append(r.Template.QuickReplies, QuickReply{
		Label:       label,
		Action:      "message",
		MessageText: messageText,
	})
	return r
}

// NewWebButton creates a button that opens a web link.
func NewWebButton(label, url string) CardButton {
	return CardButton{
		Label:      label,
		Action:     "webLink",
		WebLinkURL: url,
	}
}

// NewMessageButton creates a button that sends a chat message.
func NewMessageButton(label, messageText string) CardButton {
	return CardButton{
		Label:       label,
		Action:      "message",
		MessageText: messageText,
	}
}

// NewBlockButton creates a button that jumps to another scenario block.
func NewBlockButton(label, blockID string) CardButton {
	return CardButton{
		Label:   label,
		Action:  "block",
		BlockID: blockID,
	}
}
