- When you suspect the possibility of nil, first establish whether the design allows any path for it to become nil.
  If, by structure, a value cannot be nil, trust it and use it as is. If such a value is nil, that’s a programming
  bug—don’t hide it by wrapping it in an error; it should panic immediately.
- Functions generally should not return “absence” (nil) as a single value. If you must express absence, make it
  explicit with (T, ok bool). However, for most APIs it’s better to design them so the “absent” state is excluded altogether.
- If a switch/case grows many statements, extract the sub-logic into functions grouped by meaning. Each case should
  read at a glance: “under what condition, do what.”
- Perform existence checks only intentionally. For example, use the ok pattern only for data that can truly be missing
  (e.g., a type-owner lookup), and don’t use it for values that are internal invariants.
- Do defensive programming only at the boundaries. Validate only where input comes from outside the package; inside,
  assume invariants and write code that is simple and direct.

- Prefer diff-based comparisons with github.com/google/go-cmp/cmp so result differences are explicit.
- When an expected “snapshot” is hard to author up front, use a progressive testing strategy: first compare the got value
  against nil to surface the full structure, review it, then adopt that snapshot as the expected value.
