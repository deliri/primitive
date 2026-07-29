# Currency package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This report is the sole recon record for archived package `currency`. It
separates reusable monetary mechanism from product, provider, tax, checkout,
licensing, and display policy. Every recommendation below is grounded in the
archived implementation, its tests, or a named consumer.

## Evidence boundary

The following revisions were inspected read-only:

| Repository | Inspected revision or pin | Currency state |
| --- | --- | --- |
| `foundation_back_up_july_27th_2026` | repository HEAD `d046f7b675fcb797398d7cdc87b5504f43978056`; `HEAD:currency` tree `c473e3f77059236eee2aa926a1d68cf01d9b9171` | Full archived package |
| archived Primitive introduction | commit `80c3e48` | First currency package addition |
| archived Primitive latest currency change | commit `40ded9c104a99cbc4b0b672cd7392901b468d1eb` | Latest package-specific revision |
| archived Primitive comparison | commit `e8b7172161a4` | Same `currency`, `core/currency_constants.go`, and `core/currency_error_identity_test.go` content as archive HEAD |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`; committed Primitive pin `0df2954a2d91`; dirty worktree pin `e8b7172161a4` | Production still uses retired unsigned money and a local currency type; dirty pin exposes an incomplete migration |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e`; Primitive pin `773add8ba0fc` | Copied license source imports `currency`, but the pin and vendor tree do not provide it |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc`; Primitive pin `388e593231a2` | Same copied license/pin inconsistency as Witness |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a`; Primitive pin `3f74d8fc35b4` | No currency production or test use; dependency checksum provenance is currently broken |

The archived package itself passes:

- `go test ./currency`
- `go test -race -shuffle=on -count=2 ./currency`

Those green package-local results do not prove consumer migration or live
persistence behavior.

## Capability ownership

The archive defines one closed monetary value model:

- `currency.Code` is a closed enum for USD, EUR, GBP, CAD, AUD, JPY, CHF, NZD,
  SGD, HKD, BHD, and CLF (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/code.go:10-27`).
- `currency.Amount` atomically owns signed minor units and a validated currency
  code (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/amount.go:9-20`).
- each currency has a fixed exponent: JPY uses 0, BHD uses 3, CLF uses 4, and
  the remaining admitted currencies use 2 (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/SPEC.md:43-80`).
- construction, parsing, arithmetic, comparison, JSON, persistence projection,
  and humanization accept or return typed structures rather than loose maps
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/SPEC.md:82-117`).
- division, percentage allocation, tax calculation, exchange, and general
  rounding are explicitly excluded (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/SPEC.md:119-134`).

The package correctly owns:

- the validity of a currency code;
- the inseparability of a currency and its signed minor-unit amount;
- exact same-currency addition, subtraction, comparison, and scalar
  multiplication;
- bounded ASCII decimal parsing and canonical decimal output;
- canonical amount JSON and typed persistence projections;
- stable monetary error identities;
- deterministic, typed display projections when a caller supplies explicit
  display policy.

It must not own:

- which currencies a product, license, merchant, provider, or jurisdiction
  permits;
- whether zero or negative values are legal for a particular business action;
- checkout totals, line-item quantity policy, fee schedules, tax rules,
  discounts, refunds, or foreign exchange;
- Stripe or PayPal spelling and wire conventions;
- database collection names, schema deployment, or migration orchestration.

These are downstream contracts. They should consume `currency.Amount` and
validate their rules on their own owning types.

## Archive evidence

### Archive architecture and strengths

### Atomic monetary representation

`Amount` keeps `minorUnits int64` and `code Code` private
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/amount.go:9-12`). `Validate` rejects invalid codes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/amount.go:14-20`), and public construction is routed through
validated constructors (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/public.go:8-24`). The representation fixes
the central defect in the retired `MoneyPennies` model: an amount cannot travel
without a currency, and zero-decimal or non-two-decimal currencies are not
misrepresented as pennies.

The signed `int64` model is also materially stronger than Kernel's unsigned
money. It can represent adjustments and reversals without inventing a parallel
negative-money type. Whether a negative amount is admissible remains a
consumer-owned business rule.

### Exact checked arithmetic

Addition and subtraction require matching currency codes and reject overflow
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/amount.go:63-85`). Comparison also rejects cross-currency input
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/amount.go:99-125`). Full signed multiplication handles the
`math.MinInt64` boundary explicitly (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/amount.go:127-162`).

The corresponding tests exercise arithmetic boundaries and cross-currency
failure (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/amount_test.go:113-240`). This is useful mechanism and should
be retained, subject to the raw multiplier issue described below.

### Bounded decimal grammar

The parser accepts a deliberately narrow ASCII grammar, rejects unbounded
input, enforces the selected currency's exponent, preserves the sign contract,
and accumulates with overflow checks (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/decimal.go:11-89`). Formatting
is exact and derived from minor units rather than binary floating point
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/decimal.go:92-114`).

Semantic fuzzing covers decimal parse/format round trips
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/currency_fuzz_test.go:12-62`). This is the correct foundation for
provider adapters: providers may dictate a wire spelling, but conversion to and
from monetary value should stay exact.

### Canonical structured boundaries

Amount JSON is a typed object whose `minor_units` value is encoded as a decimal
string (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/json.go:10-18`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/json.go:69-99`). Decode uses a
temporary value and assigns to the receiver only after validation, so rejected
input does not partially mutate an existing value. Hostile JSON and receiver
preservation are tested (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/json_persistence_test.go:20-137`).

Firestore and PostgreSQL projections are distinct typed DTOs with their own
validation and inverse conversion (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/persistence.go:5-59`). Keeping
those DTOs distinct prevents an accidental informal cross-database protocol.
The implementation is sound as an in-memory projection, but live adapter proof
is absent.

### Compiler-visible errors and shared bounds

Stable currency error identities live in core
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:173-176`). Shared currency bounds live in
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/currency_constants.go:3-17`; shared currency JSON field names live in
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/json_constants.go:131-132`. Package errors preserve those identities
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/errors.go:10-31`). Callers can therefore use `errors.Is` and
`errors.As` rather than matching messages.

The amount JSON maximum is consumed by another core contract
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:155`), showing why the shared value belongs in
core rather than being copied between packages.

### Archived internal dependents

### Control state

`controlstate.PlanRequest` carries `currency.Amount` and validates it
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/plan.go:264-288`). Its business rule rejects negative amounts
while permitting zero. The private `Plan` repeats validation at its ownership
boundary (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/plan.go:308-355`), and canonical JSON embeds the typed
amount (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/plan.go:364-410`).

The specification makes the policy explicit: the value is the exact currently
billed amount, zero is valid, and negative is invalid
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:151-163`). This is a good ownership split. Currency
validates monetary structure; control state validates plan semantics.

### Receipt

`receipt.PaymentBody` carries `currency.Amount`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/payment.go:13-37`), validates it as part of its composite boundary
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/payment.go:40-60`), and emits canonical structured JSON
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/payment.go:69-132`). This is also a valid consumer pattern.

### License

The archive ledger describes license migration
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:514-555`), but the archived Primitive tree has no Go
`license` package; its `license` entry is plain text. The actual copied protocol
implementation examined here exists in Witness and Bug. The ledger is not
accepted as proof of code that is absent from the tree.

## Consumer evidence

### Kernel

Kernel contains the strongest body of commercial money behavior, but its
representation is duplicated and obsolete:

- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_currency.go:23-34` defines a local `CurrencyMinorUnit` limited to 0
  or 2 decimal places.
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_currency.go:82-130` defines a second ten-code currency enum and
  exponent mapping.
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_currency.go:133-166` defines provider-oriented lowercase labels,
  case-folding JSON, and null-as-no-op decode behavior.
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_currency.go:169-284` parses and formats unsigned provider decimals.
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_product.go:101-114` stores price as separate `MoneyPennies` and
  `PayCurrency` fields and requires a positive value.

The separate fields permit representational mismatch. They also encode the
false assumption that all money is pennies. Primitive's atomic `Amount` is the
correct replacement.

Kernel nevertheless contains downstream gems that must not be moved into the
currency package:

- The typed product catalog and positive-price policy in
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_product.go:101-170`.
- Order line quantity, line total, tax total, duplicate detection, and checked
  summation in `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/order_contracts.go:294-374` and
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/order_contracts.go:620-647`.
- Payment request validation over lines, quantities, products, amounts, and tax
  in `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:pay/pay.go:57-180`.
- Basis-point fee splitting using `bits.Mul64`, deterministic floor behavior,
  capping, and overflow checks in `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:pay/split.go:12-72`.
- PayPal major-unit decimal formatting, upper-case currency output, and
  item/total composition in `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/paypal.go:510-615`.
- Stripe lower-case currency plus integer minor units in
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/stripe.go:216-225`.
- PayPal capture parsing, positive-value policy, and `MaxInt64` bounds in
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:payapi/event.go:184-270`.
- Exact fulfillment currency matching and tax-specific amount coverage in
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:payapi/fulfill.go:172-199`.
- Firestore order conversion, validation, and inverse conversion in
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/order.go:240-293`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/order.go:313-345`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/order.go:392-435`, and
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/order.go:438-514`.

These capabilities should be migrated to consume `currency.Amount`. Product
catalog, fee, tax, fulfillment, provider, and storage policy remain in their
own packages.

Kernel's committed Primitive pin is `0df2954a2d91`, which still contains
`core.MoneyPennies`. Its dirty worktree pin is `e8b7172161a4`, where that type
has been removed. Against the dirty pin, `go test ./core -run '^$'` fails on
undefined `foundationcore.MoneyPennies` references at:

- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/checkout_failure.go:269`
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/order_contracts.go:296-297`
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/order_contracts.go:351-352`
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/numeric_contracts.go:50-58`
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_product.go:102`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_product.go:154`
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/checkout_failure.go:355`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/checkout_failure.go:386-395`
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/stripe_lifecycle_object.go:44-47`
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/order_contracts.go:620-639`

These are representative production locations; tests add further migration
surface. The same attempted pin also exposes unrelated retired temporal
references. This is evidence that pin replacement without atomic call-site
migration is unsafe.

### Witness

Witness has copied license offer code that imports Primitive `currency`:

- `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/offer.go:192-242` stores `Offer.Price` as
  `currency.Amount` and exposes typed constructors.
- `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/offer.go:244-309` constructs product-specific offers and
  validates identity and positive price.
- `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/offer_test.go:12-52` and
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/offer_test.go:394-400` construct CAD prices and assert the
  returned code.

The dependency is not buildable at the inspected pin. `go test
./protocol/license -run '^$'` fails during setup because Primitive pin
`773add8ba0fc` and the vendor tree do not contain the imported `currency`
package. It also reports missing exchange and temporal packages. The source,
pin, vendor tree, and tests are not one coherent revision.

There is also a verified business-invariant defect:
`validateOfferIdentity` (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/offer.go:285-309`) reads only
`MinorUnits`; it never requires CAD. Any valid positive non-CAD amount can pass
the production validator even though test callers and assertions assume CAD.
The allowed license currency must be a compiler-owned license/product contract,
not an accidental property of a helper.

Witness doctrine has a reusable enforcement mechanism but an obsolete rule.
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/protocol.go:213-249` flags numeric monetary field names unless
they use a `Pennies` suffix, under the rule declared at
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/rules.go:175-181`. The mechanism should evolve to reject
untyped numeric monetary domain fields altogether, except for explicitly
reviewed wire DTOs. A spelling convention for raw pennies would ratchet the
retired representation back into the system.

### Bug

Bug contains the same copied `protocol/license/offer.go` and its tests, so the
same CAD invariant defect applies. Its Primitive pin `388e593231a2` and vendor
tree likewise do not provide the imported currency package. `go test
./protocol/license -run '^$'` fails during setup for missing Primitive
currency, exchange, and temporal packages.

Bug contributes no distinct currency mechanism beyond the copied license
consumer. The correct action is to repair the owning license contract once and
consume that contract, not preserve two independently drifting copies.

### Peachfuzz

No Peachfuzz production code or tests use Primitive currency. There is no
currency capability to migrate.

`go list ./internal/...` currently fails because the downloaded content for its
pinned Primitive revision `3f74d8fc35b4` hashes to
`h1:0rSj...`, while `go.sum` expects `h1:Dton...`. This is a dependency
provenance blocker, not evidence about currency behavior. It must be resolved
before Peachfuzz can provide trustworthy integration proof, but Peachfuzz
should not gain a currency dependency merely to exercise the package.

## Strong mechanics and proof

### Architecture ratchets

The package tests its exact exported API (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/architecture_test.go:18-112`),
permitted imports (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/architecture_test.go:114-130`), and struct
inventory (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/architecture_test.go:133-164`). It also rejects
reintroduction of the retired `MoneyPennies` symbol
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/architecture_test.go:166-172`).

These ratchets are valuable. They make package growth, dependency growth, and
representation drift compiler- or test-visible.

## Defects and blockers

### 1. Currency metadata has two sources of truth

The specification requires one closed metadata table
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/SPEC.md:43-48`). The implementation instead has parallel
`codeTokens()` and `codeExponents()` tables (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/code.go:31-63`). Adding
or reordering a code can compile while the tables disagree.

Primitive must replace both with one typed `currencyDefinition` per enum
member, containing its canonical token and exponent. Validation, string
conversion, exponent lookup, JSON, and tests must all derive from that table.

### 2. Scalar multiplication accepts an untyped protocol value

`Amount.Multiply` accepts a raw `uint64` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/amount.go:87-97`). The
integer is structurally bounded but semantically unnamed: quantity, basis
points, percentages, and arbitrary multipliers are indistinguishable at the
compiler boundary.

Either remove multiplication until a real consumer requires it, or introduce a
validated typed multiplier owned by the domain that defines its meaning.
Percentage, fee, and allocation policy must not be smuggled into currency
through a raw scalar.

### 3. JSON hostility is uneven

`Code` has hostile JSON and receiver-preservation coverage
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/code_test.go:82-168`). Amount has the same
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/json_persistence_test.go:20-137`). `Order` tests cover valid JSON and
invalid enum values (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/code_test.go:170-219`) but not the same hostile
decode matrix and receiver nonmutation.

`DisplayUnit` valid JSON is tested, but rejection coverage calls
`ParseDisplayUnit` rather than hostile `UnmarshalJSON`, and receiver preservation
is not proved (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/humanize_test.go:163-211`).

All public JSON-owning types need the same typed hostile boundary proof.

### 4. The no-alias ratchet is incomplete

The architecture collector records type declarations
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/architecture_test.go:218-239`) but does not inspect
`TypeSpec.Assign.IsValid()`. It therefore has no explicit ratchet against adding
an exported or internal type alias as a compatibility shim.

The package should fail architecture tests on aliases, in addition to retaining
the specific `MoneyPennies` retirement ratchet.

### 5. ISO metadata provenance is not governed

The code set and exponents are plausible, but no authoritative ISO 4217 source
edition or update policy is recorded. A closed enum intentionally changes by
code review, yet reviewers still need a compiler-visible provenance/version
contract or generated evidence that explains why the admitted set and
exponents are correct.

### 6. Persistence proof is only in memory

The specification explicitly leaves live adapter conformance pending
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/SPEC.md:1-4`) and requires Firestore/PostgreSQL adapter evidence
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/SPEC.md:238-255`). Existing tests only round-trip typed projections
in memory (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/json_persistence_test.go:139-213`). There is no production
use of the `Firestore` or `PostgreSQL` projections outside package tests.

Primitive must either:

- provide live emulator/database round-trip and rejection proof at the real
  adapter boundary; or
- narrow the package claim to typed projection values and defer named database
  conformance until a consumer exists.

### 7. Consumer pin migrations are not atomic

Kernel, Witness, and Bug demonstrate three versions of the same failure:
production source is changed to expect new Primitive contracts while the
dependency pin, vendor tree, or remaining call sites still describe the old
world. A clean currency admission requires one revision per consumer that
updates the pin, vendor state, production call sites, tests, and ratchets
together.

### 8. License currency policy is implicit

Witness and Bug constructors accept an arbitrary `currency.Amount`; their test
helper and assertions happen to use CAD. The production validator accepts any
supported currency. The product/license owner must declare and validate its
allowed currency with a typed contract. Tests must include hostile
valid-but-wrong currencies.

### 9. Humanization has no demonstrated production demand

The humanization implementation is deterministic and typed
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/humanize.go:91-193`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/humanize.go:255-342`), but no inspected
consumer uses it. Display thresholds and units are policy-heavy. The new
Primitive should admit it only if a real external-output boundary needs it;
otherwise defer it rather than carry speculative surface area.

## Primitive 2026 ownership and DAG

### Admit to `currency`

- `Code`, `Amount`, and `Order` as closed validated types.
- One typed currency metadata definition per code.
- Exact signed minor-unit representation.
- Checked same-currency addition, subtraction, and comparison.
- Bounded exact decimal parse/format.
- Canonical typed JSON with receiver preservation.
- Typed persistence projections once their actual claim is proved.
- Humanization only when demanded by a real external-output consumer.
- Architecture ratchets for API, imports, structs, aliases, and retired money
  types.

### Admit to `core`

- Stable currency error identities.
- Shared numeric and serialized-size bounds.
- Shared JSON field names and protocol constants used by multiple packages.
- Any cross-package function-name or validation contracts.
- The governed currency metadata source/version identifier if multiple packages
  must assert it.

Core should not own `Amount` merely because it is shared. Everyone can import
the focused currency package; core owns only cross-package invariants and
identity.

### Keep in downstream owners

- Product prices and allowed product currencies.
- License price positivity and the CAD-only invariant.
- Order quantities, totals, line matching, and duplicate policy.
- Tax, fee, basis-point, discount, refund, and allocation policy.
- Stripe and PayPal wire spellings and event semantics.
- Fulfillment coverage and capture matching.
- Firestore collection/document ownership and PostgreSQL schema ownership.
- Control-state zero/negative policy.
- Receipt semantics.

Every downstream type must call `Amount.Validate()` at ingress, package
crossing, persistence, execution, and external output, then validate its own
business invariant.

## Decision rationale and conditions

The capability has real cross-consumer demand, but the archived implementation
requires the corrections below before it can support a package specification.

The archived package solves the correct foundational problem and is materially
better than Kernel's unsigned, currency-free `MoneyPennies` model. Its atomic
`Amount`, exact arithmetic, bounded decimal grammar, canonical structured
boundaries, typed error identities, and architecture ratchets are worth
carrying forward.

It must not be copied unchanged. Admission requires:

1. Replace parallel token/exponent tables with one typed metadata source.
2. Remove raw scalar multiplication or replace it with a validated typed
   contract demanded by a real consumer.
3. Add hostile JSON and receiver-preservation proof for `Order` and
   `DisplayUnit`.
4. Add an explicit no-type-alias architecture ratchet.
5. Record and enforce currency metadata provenance/version governance.
6. Prove live Firestore/PostgreSQL boundaries or narrow the persistence claim.
7. Migrate each consumer atomically with its Primitive pin and vendor state.
8. Convert Kernel's products, orders, provider adapters, fees, and storage to
   consume atomic `currency.Amount` while leaving their policy downstream.
9. Make the Witness/Bug allowed license currency a typed invariant and add
   valid-but-wrong-currency hostile tests.
10. Change Witness doctrine from blessing `*Pennies` numeric fields to rejecting
    untyped domain money outside explicitly reviewed wire DTOs.
11. Resolve Peachfuzz's Primitive checksum provenance before accepting any
    integration evidence; do not add a currency dependency without a use case.

Until those corrections and consumer migrations are independently proved, the
package remains a strong candidate rather than an admitted Primitive 2026
contract.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
