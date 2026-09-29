# Primitive v2026.1.99

Exchange adds `WriteProduced`, a direct response writer effect for incremental
producers. It applies typed status, content type, and headers before invoking the
producer, observes cancellation and short writes, and retains producer/write
errors. Response size has no aggregate byte ceiling. The producer controls its
own working memory; Exchange does not buffer its output or start a pipe goroutine.

The Exchange writer fuzz inventory exercises the new public door alongside the
existing JSON, bounded, and stream writers. A regression transfers more than
two megabytes in fixed chunks and checks the response contract and error
identity after partial output.
