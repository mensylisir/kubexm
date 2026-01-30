#!/bin/bash
# Script to fix Step Run method signatures

STEP_DIR="/mydata/workspace/kubexm/pkg/step"

# Find all Go files with the wrong signature
find "$STEP_DIR" -name "*.go" -type f ! -path "*/test*" | while read -r file; do
    if grep -q "func (s \*[A-Z][a-zA-Z]*Step) Run(ctx runtime.ExecutionContext) error" "$file"; then
        echo "Fixing: $file"
        
        # Replace the function signature
        sed -i 's/func (s \*\(Step\)) Run(ctx runtime\.ExecutionContext) error {/func (s *\1) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {/' "$file"
        
        # Replace "return err" with "return nil, err" at the end of the function
        # This is a simplistic approach - may need manual review
        sed -i 's/return err$/return nil, err/' "$file"
        
        # Add "return result, nil" at the end if not present
        # This needs more sophisticated pattern matching
    fi
done

echo "Done fixing Step Run signatures"
