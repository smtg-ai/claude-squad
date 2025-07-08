#!/bin/bash
set -euo pipefail

DEST_DIR="_pro"

echo "Copying *_pro.go files into '$DEST_DIR/'..."

# Find all *_pro.go files, excluding the 'pro/' directory itself. Copy these files into the pro directory and
# proper subdirectories.
find . -type f -name '*_pro.go' ! -path "./$DEST_DIR/*" | while read -r src_file; do
    # Strip the leading ./ and get the relative path
    relative_path="${src_file#./}"

    # Compute the destination path inside 'pro/'
    dest_path="$DEST_DIR/$relative_path"

    # Ensure the destination directory exists
    mkdir -p "$(dirname "$dest_path")"

    # Copy the file
    cp "$src_file" "$dest_path"

    echo "Copied: $src_file → $dest_path"
done

echo "✅ Done copying *_pro.go files to '$DEST_DIR/'"
