# Operate Sales Pipeline

Use this skill when an operator wants help validating or running the sales
pipeline extension.

## Default workflow

1. Open the board and confirm the default stages exist.
2. Create a test opportunity and move it across at least two stages.
3. Verify the `sales-deal-intake` form still exists and auto-creates a case.
4. Run:
   - `mbr extensions validate --id EXTENSION_ID`
   - `mbr extensions monitor --id EXTENSION_ID`

## Notes

- B2B/B2C mode currently comes from extension config.
- The current board focuses on deal flow and stage totals, not full CRM depth.
