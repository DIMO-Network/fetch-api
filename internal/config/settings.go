// Package config holds application configuration settings.
package config

import "github.com/DIMO-Network/clickhouse-infra/pkg/connect/config"

// Settings contains the application config.
type Settings struct {
	Port                      int             `yaml:"PORT"`
	MonPort                   int             `yaml:"MON_PORT"`
	GRPCPort                  int             `yaml:"GRPC_PORT"`
	TokenExchangeJWTKeySetURL string          `yaml:"TOKEN_EXCHANGE_JWK_KEY_SET_URL"`
	TokenExchangeIssuer       string          `yaml:"TOKEN_EXCHANGE_ISSUER_URL"`
	VehicleNFTAddress         string          `yaml:"VEHICLE_NFT_ADDRESS"`
	CloudEventBucket          string          `yaml:"CLOUDEVENT_BUCKET"`
	EphemeralBucket           string          `yaml:"EPHEMERAL_BUCKET"`
	S3AWSRegion               string          `yaml:"S3_AWS_REGION"`
	S3AWSAccessKeyID          string          `yaml:"S3_AWS_ACCESS_KEY_ID"`
	S3AWSSecretAccessKey      string          `yaml:"S3_AWS_SECRET_ACCESS_KEY"`
	Clickhouse                config.Settings `yaml:",inline"`
}
