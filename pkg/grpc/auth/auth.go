package auth

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/DIMO-Network/nameindexer"
	"github.com/DIMO-Network/shared/middleware/privilegetoken"
	"github.com/DIMO-Network/shared/privileges"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// TokenClaimsKey is the key used to store the token claims in the fiber context
	TokenClaimsKey = "tokenClaims"
)

// AllOf creates a middleware that checks if the token contains all the required privileges
// this middleware also checks if the token is for the correct contract and token ID
func AllOf(contract common.Address, privilegeIDs []privileges.Privilege) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return checkAllPrivileges(c, contract, privilegeIDs)
	}
}

func checkAllPrivileges(ctx *fiber.Ctx, contract common.Address, privilegeIDs []privileges.Privilege) error {
	// This checks that the privileges are for the token specified by the path variable and the contract address is correct.
	err := validateTokenIDAndAddress(ctx, contract)
	if err != nil {
		return err
	}

	claims := getTokenClaim(ctx)
	for _, v := range privilegeIDs {
		if !slices.Contains(claims.PrivilegeIDs, v) {
			return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized! Token does not contain required privileges")
		}
	}

	return ctx.Next()
}

type subjectData struct {
	subject string `json:"subject"`
}

func validateTokenIDAndAddress(ctx *fiber.Ctx, contract common.Address) error {
	return fiber.NewError(fiber.StatusUnauthorized, "Endpoint not supported")
	claims := getTokenClaim(ctx)
	var subBody subjectData
	err := ctx.BodyParser(&subBody)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("Failed to parse subject from request body: %v", err))
	}
	subDID, err := nameindexer.DecodeNFTDIDIndex(subBody.subject)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("Failed to decode subject: %v", err))
	}
	if claims.ContractAddress != contract || claims.ContractAddress != subDID.ContractAddress {
		return fiber.NewError(fiber.StatusUnauthorized, fmt.Sprintf("Provided token is for the wrong contract: %s", claims.ContractAddress))
	}
	claimTokenId, err := strconv.ParseUint(claims.TokenID, 0, 32)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("Failed to parse token ID from token claims: %v", err))
	}
	if uint32(claimTokenId) != subDID.TokenID {
		return fiber.NewError(fiber.StatusUnauthorized, fmt.Sprintf("Provided token is for the wrong token ID: %s", claims.TokenID))
	}

	return nil
}

func getTokenClaim(ctx *fiber.Ctx) *privilegetoken.Token {
	token := ctx.Locals("user").(*jwt.Token)
	claim, ok := token.Claims.(*privilegetoken.Token)
	if !ok {
		panic("TokenClaimsKey not found in fiber context")
	}
	return claim
}
