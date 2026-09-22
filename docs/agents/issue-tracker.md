# Issue tracker: Local Markdown

Issues and specifications for this repository live as markdown files under `.scratch/`.

- One effort uses one directory: `.scratch/<effort>/`
- The map is `.scratch/<effort>/map.md`
- Child decision tickets are `.scratch/<effort>/issues/NN-<slug>.md`
- `Status:` is `open`, `claimed`, or `resolved`
- `Type:` is `research`, `prototype`, `grilling`, or `task`
- `Blocked by:` lists ticket numbers whose status must be `resolved`

A ticket is claimed by setting `Status: claimed` before working on it. Resolve it by adding an `## Answer`, setting `Status: resolved`, and adding a context pointer to the map's Decisions so far.
