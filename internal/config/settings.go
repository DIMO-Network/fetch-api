// Package config holds application configuration settings.
package config

import (
	"github.com/DIMO-Network/clickhouse-infra/pkg/connect/config"
	"github.com/ethereum/go-ethereum/common"
)

// Settings contains the application config.
type Settings struct {
	Port                      int            `env:"PORT"`
	MonPort                   int            `env:"MON_PORT"`
	GRPCPort                  int            `env:"GRPC_PORT"`
	EnablePprof               bool           `env:"ENABLE_PPROF"`
	TokenExchangeJWTKeySetURL string         `env:"TOKEN_EXCHANGE_JWK_KEY_SET_URL"`
	TokenExchangeIssuer       string         `env:"TOKEN_EXCHANGE_ISSUER_URL"`
	VehicleNFTAddress         common.Address `env:"VEHICLE_NFT_ADDRESS"`
	ChainID                   string         `env:"CHAIN_ID"`
	CloudEventBucket          string         `env:"CLOUDEVENT_BUCKET"`
	EphemeralBucket           string         `env:"EPHEMERAL_BUCKET"`
	VCBucket                  string         `env:"VC_BUCKET"`
	S3AWSRegion               string         `env:"S3_AWS_REGION"`
	S3AWSAccessKeyID          string         `env:"S3_AWS_ACCESS_KEY_ID"`
	S3AWSSecretAccessKey      string         `env:"S3_AWS_SECRET_ACCESS_KEY"`
	Clickhouse                config.Settings
}
