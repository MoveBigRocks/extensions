# Operate Community Feature Requests

Use this skill when an operator wants help validating or running the community
feature requests extension.

## Default workflow

1. Visit `/community/ideas` and submit a test request.
2. Open the request detail page and confirm voting works once per browser key.
3. Open the admin dashboard and update the idea status.
4. Run:
   - `mbr extensions validate --id EXTENSION_ID`
   - `mbr extensions monitor --id EXTENSION_ID`

## Notes

- Voting is intentionally lightweight in the first slice.
- Comments and richer public roadmap linking are follow-on work.
