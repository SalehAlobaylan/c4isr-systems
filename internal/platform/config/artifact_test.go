package config

import "testing"

func TestArtifactConfigurationMustBeComplete(t *testing.T) {
	cfg := Config{Environment: EnvironmentTest}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.ArtifactS3Bucket = "only-bucket"
	if cfg.Validate() == nil {
		t.Fatal("partial storage settings ignored")
	}
	cfg.ArtifactS3Endpoint = "https://storage.example.invalid"
	cfg.ArtifactS3AccessKey = "fixture"
	cfg.ArtifactS3SecretKey = "fixture"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
