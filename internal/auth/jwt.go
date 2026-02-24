package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/DIMO-Network/fetch-api/internal/graph"
	"github.com/DIMO-Network/token-exchange-api/pkg/tokenclaims"
	"github.com/rs/zerolog"
)

// FetchClaim wraps tokenclaims.Token for auth0 validator and injects into graph.ClaimsContextKey.
type FetchClaim struct {
	tokenclaims.Token
}

// Validate implements validator.CustomClaims.
func (*FetchClaim) Validate(context.Context) error {
	return nil
}

// NewJWTMiddleware creates JWT middleware with token-exchange issuer/JWKS.
func NewJWTMiddleware(issuer, jwksURI string) (*jwtmiddleware.JWTMiddleware, error) {
	issuerURL, err := url.Parse(issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuer URL: %w", err)
	}
	var opts []jwks.ProviderOption
	if jwksURI != "" {
		keysURI, err := url.Parse(jwksURI)
		if err != nil {
			return nil, fmt.Errorf("failed to parse jwksURI: %w", err)
		}
		opts = append(opts, jwks.WithCustomJWKSURI(keysURI))
	}
	provider := jwks.NewCachingProvider(issuerURL, 1*time.Minute, opts...)
	newCustomClaims := func() validator.CustomClaims {
		return &FetchClaim{}
	}
	jwtValidator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{"dimo.zone"},
		validator.WithCustomClaims(newCustomClaims),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}
	middleware := jwtmiddleware.New(
		jwtValidator.ValidateToken,
		jwtmiddleware.WithErrorHandler(ErrorHandler),
		jwtmiddleware.WithCredentialsOptional(true),
	)
	return middleware, nil
}

// AddClaimHandler puts *tokenclaims.Token into graph.ClaimsContextKey for resolvers.
func AddClaimHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetValidatedClaims(r.Context())
		if !ok || claims.CustomClaims == nil {
			next.ServeHTTP(w, r)
			return
		}
		fc, ok := claims.CustomClaims.(*FetchClaim)
		if !ok {
			zerolog.Ctx(r.Context()).Error().Msg("Could not cast claims to FetchClaim")
			jwtmiddleware.DefaultErrorHandler(w, r, jwtmiddleware.ErrJWTMissing)
			return
		}
		r = r.Clone(context.WithValue(r.Context(), graph.ClaimsContextKey{}, &fc.Token))
		next.ServeHTTP(w, r)
	})
}

// GetValidatedClaims returns validated claims from context.
func GetValidatedClaims(ctx context.Context) (*validator.ValidatedClaims, bool) {
	claim := ctx.Value(jwtmiddleware.ContextKey{})
	if claim == nil {
		return nil, false
	}
	vc, ok := claim.(*validator.ValidatedClaims)
	return vc, ok
}

// ErrorHandler logs JWT validation errors and calls the default handler.
func ErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	zerolog.Ctx(r.Context()).Error().Err(err).Msg("error validating token")
	jwtmiddleware.DefaultErrorHandler(w, r, err)
}
