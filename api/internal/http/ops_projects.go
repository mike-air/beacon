package http

// Projects and the board.
//
// One thing here is not like the others: listing projects has two response
// shapes behind the `new_board_ui` experiment, and the SERVER decides which
// one a caller gets. The v2 body is the v1 body plus `board: "v2"`, and the
// client renders whichever board that marker names. Putting the decision in
// the response rather than in a flag the client reads keeps one source of
// truth about which arm a user is in.

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"beacon/internal/auth"
	"beacon/internal/flags"
	"beacon/internal/projects"
)

type ListProjectsInput struct {
	OrgPath
	Paging
}

// ListProjectsOutput carries the optional experiment marker. `omitempty` is
// what makes it optional in the generated client too: absent means v1.
type ListProjectsOutput struct {
	Body struct {
		Items  []projects.Project `json:"items" nullable:"false"`
		Limit  int                `json:"limit"`
		Offset int                `json:"offset"`
		Board  string             `json:"board,omitempty" enum:"v2" doc:"Present only on the v2 arm of the new_board_ui experiment. Absent means v1."`
	}
}

type CreateProjectInput struct {
	OrgPath
	IdempotencyHeader
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"200" required:"true"`
	}
}

type ProjectInput struct{ ProjectPath }

type UpdateProjectInput struct {
	ProjectPath
	IdempotencyHeader
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"200" required:"true"`
	}
}

type ProjectOutput struct {
	Status int
	Body   projects.Project
}

// NoContentOutput is the shape of a successful delete: a 204 and nothing else.
type NoContentOutput struct{}

func (s *Server) registerProjects(api huma.API, g gates) {
	huma.Register(api, huma.Operation{
		OperationID: "list-projects",
		Method:      http.MethodGet,
		Path:        "/v1/orgs/{orgID}/projects",
		Summary:     "Projects in the organisation",
		Tags:        []string{"projects"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *ListProjectsInput) (*ListProjectsOutput, error) {
		list, err := s.projects.List(ctx, in.OrgID, in.Limit, in.Offset)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		out := &ListProjectsOutput{}
		// Same nil-slice rule as listBody; this envelope carries an extra
		// field so it cannot use the shared constructor.
		if list == nil {
			list = []projects.Project{}
		}
		out.Body.Items = list
		out.Body.Limit = in.Limit
		out.Body.Offset = in.Offset

		// The flag gates the feature; the experiment measures it. They share a
		// key on purpose, so there is exactly one thing being discussed.
		userID, _ := auth.UserIDFrom(ctx)
		if s.flags.Enabled(ctx, FlagNewBoardUI, flags.Subject{UserID: userID, OrgID: in.OrgID}) {
			switch s.experiments.VariantFor(ctx, FlagNewBoardUI, userID) {
			case "treatment":
				out.Body.Board = "v2"
			}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-project",
		Method:        http.MethodPost,
		Path:          "/v1/orgs/{orgID}/projects",
		Summary:       "Create a project",
		Tags:          []string{"projects"},
		Security:      []map[string][]string{{"bearerAuth": {}}},
		DefaultStatus: http.StatusCreated,
		Middlewares:   g.orgScoped,
	}, func(ctx context.Context, in *CreateProjectInput) (*ProjectOutput, error) {
		p, err := s.projects.Create(ctx, in.OrgID, in.Body.Name)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &ProjectOutput{Status: http.StatusCreated, Body: p}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-project",
		Method:      http.MethodGet,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}",
		Summary:     "One project",
		Tags:        []string{"projects"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *ProjectInput) (*ProjectOutput, error) {
		p, err := s.projects.Get(ctx, in.OrgID, in.ProjectID)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &ProjectOutput{Status: http.StatusOK, Body: p}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-project",
		Method:      http.MethodPatch,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}",
		Summary:     "Rename a project",
		Tags:        []string{"projects"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *UpdateProjectInput) (*ProjectOutput, error) {
		p, err := s.projects.Update(ctx, in.OrgID, in.ProjectID, in.Body.Name)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &ProjectOutput{Status: http.StatusOK, Body: p}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-project",
		Method:        http.MethodDelete,
		Path:          "/v1/orgs/{orgID}/projects/{projectID}",
		Summary:       "Delete a project and everything in it",
		Tags:          []string{"projects"},
		Security:      []map[string][]string{{"bearerAuth": {}}},
		DefaultStatus: http.StatusNoContent,
		Middlewares:   g.orgScoped,
	}, func(ctx context.Context, in *ProjectInput) (*NoContentOutput, error) {
		if err := s.projects.Delete(ctx, in.OrgID, in.ProjectID); err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &NoContentOutput{}, nil
	})
}
