# Prompt bundle integrity

Date: 2026-10-18.

The embedded bundle at server/internal/engineer/prompts/ is byte-identical to docs/rework/prompts/v1/ for every file:

    for f in docs/rework/prompts/v1/*; do cmp -s "$f" "server/internal/engineer/prompts/$(basename "$f")"; done

All comparisons match. Manifest hashes verified at runtime by engineer.VerifyBundle, exercised by TestPromptBundleMatchesManifest.

base.md SHA-256: 5bc3e1997a251d350e5432f59972a4d8586c44e32e5f520487eab880b85c0264

No prompt wording was edited or regenerated.
