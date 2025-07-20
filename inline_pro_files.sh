#!/bin/bash
set -euo pipefail

# Root of the pro version files
PRO_DIR="_pro"

echo "copying *_pro.go and *_pro_test.go files from '$PRO_DIR'..."

# Find all *_pro.go and *_pro_test.go files under the pro directory and copy to the corresponding non-pro directory. Remember
# to add !pro and pro build flags to the basic and pro .go files respectively
find "$PRO_DIR" -type f \( -name '*_pro.go' -o -name '*_pro_test.go' \) | while read -r pro_file; do
    # Strip off the pro/ prefix
    relative_path="${pro_file#$PRO_DIR/}"

    # Compute the destination path in the current directory
    dest_path="./${relative_path}"

    # Ensure the destination directory exists
    dest_dir="$(dirname "$dest_path")"
    mkdir -p "$dest_dir"

    # Copy the file
    cp "$pro_file" "$dest_path"
    echo "Copied: $pro_file → $dest_path"
done

echo "✅ All *_pro.go and *_pro_test.go files copied."
