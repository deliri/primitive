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
	// GitHubRecursiveTreeMaximumEntries is GitHub's recursive-tree entry ceiling.
	// Source: https://docs.github.com/en/rest/git/trees?apiVersion=2026-03-10#get-a-tree
	GitHubRecursiveTreeMaximumEntries = 100_000
	// GitHubRecursiveTreeMaximumBytes is GitHub's documented 7 MB recursive-tree ceiling.
	// GitHub states this limit in decimal megabytes.
	// Source: https://docs.github.com/en/rest/git/trees?apiVersion=2026-03-10#get-a-tree
	GitHubRecursiveTreeMaximumBytes = 7_000_000
	// GitHubContentsInlineMaximumBytes is the largest file for which the contents
	// endpoint supports every documented media type, including its JSON base64 form.
	// Source: https://docs.github.com/en/rest/repos/contents?apiVersion=2026-03-10#get-repository-content
	GitHubContentsInlineMaximumBytes = 1_000_000

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
	// GitHubInstallationTokenResponseCustodyMaximumBytes is Primitive's bounded
	// response budget for the documented installation-token response shape.
	// Source: https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app
	GitHubInstallationTokenResponseCustodyMaximumBytes = 16 * 1024
	// GitHubCommitResponseCustodyMaximumBytes is Primitive's bounded response
	// budget for one commit-list result; GitHub publishes no response byte maximum.
	// Source: https://docs.github.com/en/rest/commits/commits?apiVersion=2026-03-10#list-commits
	GitHubCommitResponseCustodyMaximumBytes = 256 * 1024
	// GitHubTagPageResponseCustodyMaximumBytes is Primitive's bounded response
	// budget for one provider-capped page of repository tags.
	// Source: https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#list-repository-tags
	GitHubTagPageResponseCustodyMaximumBytes = 1024 * 1024
	// GitHubContentsResponseCustodyMaximumBytes includes base64 expansion and
	// bounded metadata for one provider-supported inline contents response.
	// Source: https://docs.github.com/en/rest/repos/contents?apiVersion=2026-03-10#get-repository-content
	GitHubContentsResponseCustodyMaximumBytes = 2 * 1024 * 1024
	// GitHubOperationCustodyTimeoutSeconds is Primitive's per-operation wall-clock
	// budget; GitHub publishes no operation timeout contract.
	GitHubOperationCustodyTimeoutSeconds = 20
)
