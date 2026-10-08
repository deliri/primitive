# Explicit removal durability

RemovalRequest now requires a closed RemovalDurability enum. Unknown and zero policies refuse before mutation. Ephemeral intent unlinks without parent synchronization; durable intent preserves parent synchronization and cleanup errors. Existing Primitive producers explicitly request durable removal. There is one current Remove API and no default compatibility behavior.

Real native permission tests permit unlink but refuse opening the parent for synchronization. These tests distinguish successful ephemeral cleanup from a durable cleanup refusal after actual deletion. Invalid policies preserve exact native bytes. Mutations that always synchronized or never synchronized each fail their relevant case. Semantic fuzz validates every raw enum value, actual namespace change, preserved rejected bytes, and cancellation identity; its bounded fixture does not limit production streams.

The broader affected-package race run exposed an intermittent repository-verification failure under declared local stat-cache settings. The failed run and subsequently green filtered probe remain historical evidence. Repository verification now refuses declared local stat-cache configuration as uncommitted verification input; it observes the first config value byte without retaining the text. Default, empty, and removed declarations receive distinct regression coverage. This conservative refusal does not claim that timestamp metadata is a complete source-content hash.

No independent acceptance or final global gates are claimed. Source catalogues retain the earlier failed attempt and the final source independently. Successful package scopes, retries, build failures, skips, and whole-command outcomes remain visible in execution accounting.
