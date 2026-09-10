package core

const (
	// GitHubAPIHost is the GitHub REST API authority.
	// Source: https://docs.github.com/en/rest/using-the-rest-api/getting-started-with-the-rest-api?apiVersion=2026-03-10
	GitHubAPIHost = "api.github.com"
	// GitHubAPIVersion is the dated REST contract sent on every API request.
	// Source: https://docs.github.com/en/rest/about-the-rest-api/api-versions?apiVersion=2026-03-10
	GitHubAPIVersion = "2026-03-10"

	// GitHubTagPageMaximumEntries is GitHub's documented per_page ceiling.
	// Source: https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#list-repository-tags
	GitHubTagPageMaximumEntries = 100
	// GitHubRawContentMediaType selects the provider's streaming file representation.
	// Directories retain application/json and cannot pass as raw file bytes.
	// Source: https://docs.github.com/en/rest/repos/contents#get-repository-content
	GitHubRawContentMediaType = "application/vnd.github.raw+json"

	// GitHubAppJWTMaximumLifetimeSeconds is GitHub's maximum JWT lifetime.
	// Source: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app
	GitHubAppJWTMaximumLifetimeSeconds = 10 * 60
	// GitHubAppJWTClockSkewSeconds is GitHub's recommended issued-at backdating.
	// Source: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app
	GitHubAppJWTClockSkewSeconds = 60
	// GitHubAppPrivateKeyCustodyMaximumBytes is Primitive's memory-custody budget;
	// GitHub documents the PEM key requirement but publishes no aggregate byte maximum.
	// Source: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps
	GitHubAppPrivateKeyCustodyMaximumBytes = 64 * 1024
	// GitHubOperationCustodyTimeoutSeconds is Primitive's per-operation wall-clock
	// budget; GitHub publishes no operation timeout contract.
	GitHubOperationCustodyTimeoutSeconds = 20
)
