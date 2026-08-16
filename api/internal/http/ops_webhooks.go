package http

// Outgoing webhooks.
//
// Owner and admin only, because a webhook is an outbound channel: whoever
// registers one decides where this organisation's activity gets sent.
//
// The signing secret is returned in full exactly once, in the create response,
// and never again. That is a deliberate property of the API and it has to be
// in the contract, or a generated client will not know to show it — and a
// client that quietly drops it leaves the user unable to verify a single
// delivery for the life of the webhook.

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"beacon/internal/webhooks"
)

type ListWebhooksInput struct {
	OrgPath
	Paging
}

type ListWebhooksOutput struct {
	Body ListBody[webhooks.Webhook]
}

type RegisterWebhookInput struct {
	OrgPath
	IdempotencyHeader
	Body struct {
		URL string `json:"url" format:"uri" maxLength:"2000" required:"true"`
		// Empty means every event.
		Events []string `json:"events" doc:"Empty means every event. Beacon publishes task.created, task.updated and task.deleted."`
	}
}

type WebhookOutput struct {
	Status int
	Body   webhooks.Webhook
}

type DeleteWebhookInput struct {
	OrgPath
	WebhookID string `path:"webhookID" format:"uuid"`
}

func (s *Server) registerWebhooks(api huma.API, g gates) {
	sec := []map[string][]string{{"bearerAuth": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "list-webhooks",
		Method:      http.MethodGet,
		Path:        "/v1/orgs/{orgID}/webhooks",
		Summary:     "Registered webhooks",
		Tags:        []string{"webhooks"},
		Security:    sec,
		Middlewares: g.orgAdmin,
	}, func(ctx context.Context, in *ListWebhooksInput) (*ListWebhooksOutput, error) {
		list, err := s.webhooks.List(ctx, in.OrgID)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &ListWebhooksOutput{Body: page(list, in.Limit, in.Offset)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "register-webhook",
		Method:      http.MethodPost,
		Path:        "/v1/orgs/{orgID}/webhooks",
		Summary:     "Register a webhook",
		Description: "The response is the only time the signing secret is returned " +
			"in full. Beacon cannot show it again.",
		Tags:          []string{"webhooks"},
		Security:      sec,
		DefaultStatus: http.StatusCreated,
		Middlewares:   g.orgAdmin,
	}, func(ctx context.Context, in *RegisterWebhookInput) (*WebhookOutput, error) {
		w, err := s.webhooks.Register(ctx, in.OrgID, in.Body.URL, in.Body.Events)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &WebhookOutput{Status: http.StatusCreated, Body: w}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-webhook",
		Method:        http.MethodDelete,
		Path:          "/v1/orgs/{orgID}/webhooks/{webhookID}",
		Summary:       "Delete a webhook",
		Tags:          []string{"webhooks"},
		Security:      sec,
		DefaultStatus: http.StatusNoContent,
		Middlewares:   g.orgAdmin,
	}, func(ctx context.Context, in *DeleteWebhookInput) (*NoContentOutput, error) {
		if err := s.webhooks.Delete(ctx, in.OrgID, in.WebhookID); err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &NoContentOutput{}, nil
	})
}
