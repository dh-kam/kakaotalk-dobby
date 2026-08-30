package user

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dh-kam/kakao-bot/pkg/kakao"
	"github.com/samber/lo"
)

// MeRequest holds parameters for retrieving current user profile.
type MeRequest struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	Out          io.Writer
}

// MeUseCase retrieves the authenticated Kakao user profile.
type MeUseCase struct{}

func NewMeUseCase() *MeUseCase {
	return &MeUseCase{}
}

func (uc *MeUseCase) Execute(ctx context.Context, req MeRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}

	client := kakao.NewClient(kakao.ClientConfig{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
		TokenStore:   kakao.NewFileTokenStore(req.TokenPath),
	})

	profile, err := client.User().GetMe(ctx)
	if err != nil {
		return fmt.Errorf("get user profile: %w", err)
	}

	fmt.Fprintln(out, "Kakao User Profile:")
	fmt.Fprintf(out, "  ID:            %d\n", profile.ID)
	if profile.KakaoAccount != nil {
		acc := profile.KakaoAccount
		if acc.Profile != nil {
			fmt.Fprintf(out, "  Nickname:      %s\n", acc.Profile.Nickname)
			if acc.Profile.ProfileImageURL != "" {
				fmt.Fprintf(out, "  Profile Image: %s\n", acc.Profile.ProfileImageURL)
			}
		}
		if acc.Email != "" {
			fmt.Fprintf(out, "  Email:         %s\n", acc.Email)
		}
		if acc.PhoneNumber != "" {
			fmt.Fprintf(out, "  Phone:         %s\n", acc.PhoneNumber)
		}
	}
	if profile.ConnectedAt != "" {
		fmt.Fprintf(out, "  Connected At:  %s\n", profile.ConnectedAt)
	}

	return nil
}

// ShippingAddressRequest holds parameters for querying user shipping addresses.
type ShippingAddressRequest struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	FromUpdated  int
	PageSize     int
	Out          io.Writer
}

// ShippingAddressUseCase retrieves shipping addresses for the user.
type ShippingAddressUseCase struct{}

func NewShippingAddressUseCase() *ShippingAddressUseCase {
	return &ShippingAddressUseCase{}
}

func (uc *ShippingAddressUseCase) Execute(ctx context.Context, req ShippingAddressRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}

	client := kakao.NewClient(kakao.ClientConfig{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
		TokenStore:   kakao.NewFileTokenStore(req.TokenPath),
	})

	resp, err := client.User().GetShippingAddresses(ctx, req.FromUpdated, req.PageSize)
	if err != nil {
		return fmt.Errorf("get shipping addresses: %w", err)
	}

	if len(resp.ShippingAddresses) == 0 {
		fmt.Fprintln(out, "No registered shipping addresses found.")
		return nil
	}

	fmt.Fprintf(out, "Found %d Shipping Address(es) for User %d:\n", len(resp.ShippingAddresses), resp.UserID)
	lo.ForEach(resp.ShippingAddresses, func(addr kakao.ShippingAddress, idx int) {
		defaultBadge := ""
		if addr.IsDefault {
			defaultBadge = " (Default)"
		}
		fmt.Fprintf(out, "\n[%d] %s%s\n", idx+1, addr.Name, defaultBadge)
		fmt.Fprintf(out, "    Receiver: %s (%s)\n", addr.ReceiverName, addr.ReceiverPhone1)
		fmt.Fprintf(out, "    Address:  [%s] %s %s\n", addr.ZoneNumber, addr.BaseAddress, addr.DetailAddress)
	})

	return nil
}
