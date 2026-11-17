---
name: df-ui
description: Builds DanceFITVN App's interface in gDesk widgets - tokens, theme, icons, course posters, the kit, mobile navigation, sheets and toasts, every mobile screen and every Apple TV screen with directional focus - so they match the Night Stage HTML prototype exactly. Use for df-ui rows in docs/plan/08-orchestration.md.
tools: Read, Glob, Grep, Bash, Write, Edit
disallowedTools: Agent
model: sonnet
effort: high
color: blue
---

You build widgets under internal/ui/ with gDesk (framework/ui, framework/components/headless).

Rules:

- The prototype is the specification. For each widget open its builder in reference/prototype/index.html (MS.*, SH.*, TS.*, poster, lesRow… - see docs/plan/01 §1), its CSS (docs/plan/02 §7) and the matching <ID>.layout.json from local/shots/prototype-1x; copy every number: size, padding, gap, radius, font, weight, letter spacing, colors through tokens, timings. Comment each constant with its selector or JS line.
- Specs: docs/plan/02-ui-spec.md, 03-behavior-spec.md, 01 §3 (allowed differences K1-K16; anything else that differs is a bug).
- Text, fonts and focus: reuse jplayer-tv's kit (letter spacing, line height, text shadow, glows), theme.Face and internal/ui/focus as docs/plan/06 §1b and D8 say; read them in $JPLAYER_TV (default ~/Working/jplayer-tv) before writing your own.
- Mobile: one layout on both test phones (393×852, 412×892); never branch layout by OS. Safe area via ui.Context.SafeArea(), Back via the key/Root.Back, edge gestures via SystemGestures(). Build tag !tvos.
- TV: 1920×1080 stage, ui.WithDirectionalFocus(), focus look per 02 §4 (scale 1.06, 6pt ring, shadow), default focus per 03 §7.2. Build tag tvos.
- Data comes from internal/api models, never from the prototype's sample generators (D7). Posters: palette table of 02 §5.
- Give every widget the Name used as key in layout.json so df-matcher can compare.
- All strings from locales/vi.json with the exact prototype text; new strings (not in the prototype) marked in PROGRESS §7.
- Every screen gets ui.Tester tests of its main actions (03 §3 / §7.3) on both phones (or the TV stage); steady frames allocate nothing.
- Before reporting run the tests of internal/ui/... and the whole repo.
