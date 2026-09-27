# UX audit: queue and navigation

Baseline: main `920a71a` (PR #19). September 27, 2026.

## Findings and changes

| Finding | Severity | Resolution in this PR |
| --- | --- | --- |
| Dashboard and Admin queue show the same jobs as unrelated top-level destinations | High | One Queue destination with Board/List views; settings separated from daily work |
| Dashboard links send members into an admin-only route, despite queue APIs allowing members | High | Shared queue/details available to signed-in members; People/Printers remain admin-only |
| Inspect changes global selection without updating the URL; Close leaves URL selection behind | High | URL owns selection; dedicated detail page and explicit parent link; no sticky selected-item global |
| Filters disappear after saves, refresh, or navigation | High | View and filter are URL parameters preserved through detail links and the parent link |
| Board counts all jobs but only shows seven per column | High | Render all jobs; all six statuses are reachable in either view |
| Submission leaves users on a reset form without a next step | Medium | Open the created print; replace the submitted form's history entry; disable submit while pending |
| Intake presents every operational field at once | Medium | Title, source links/files and notes first; advanced print options in a disclosure |
| Thumbnail diagnostics compete with job actions on every card | Medium | Remove diagnostic badges from the list footer; one View print action per card; source diagnostics remain in details |
| Large stacked mobile sidebar pushes content down | Medium | Compact wrapped navigation and account row; viewport overflow checks |
| Unknown routes silently masquerade as Dashboard | Medium | Explicit not-found page and return link; old dashboard/admin-queue URLs normalize in place |
| Async updates can repaint a page after the user leaves | Medium | Detached-view checks and item IDs captured by handlers rather than mutable global selection |

## Navigation contract

- `#queue?view=board` is home; List is a presentation choice, not another queue.
- `#queue?view=list&status=queued&item=17` opens that print with its parent context.
- Browser Back/Forward follows URL history; reload and direct links restore selection.
- Back to queue removes only `item`, so it also works for a direct link with no prior app history.
- `#dashboard` and `#admin-queue?item=17` remain supported aliases.
- New print is the sole intake destination. Printers and People are admin settings.
- No API permission changes, database migrations, or production deployment.

## Verification

`npm test` runs URL-state regression checks. `npm run test:ui` runs Chromium against
the real static frontend and mocked API responses: member access, 9-job columns,
Back/Forward, reload, filters, saves, notes, intake completion, missing routes/items,
and a 390px mobile viewport. Screenshots are CI artifacts.

These browser tests exercise frontend behavior, not live backend integration. The
existing Go and Docker clean-volume CI jobs remain in place. Local Go is unavailable
on the audit machine, so backend/build/container validation is delegated to CI.

## Remaining follow-ups (not silently bundled into this cleanup)

- Preserve drafts / warn before leaving unsaved forms.
- Search, pagination or virtualization once queues outgrow the current all-items API.
- Printer/account forms still need consistent pending/error feedback.
- Thumbnail reliability remains tracked by #18; this PR does not change fetching.
- The historical deployed-container problem needs a fresh runtime reproduction;
  the latest main image-publish and CI runs were successful.
