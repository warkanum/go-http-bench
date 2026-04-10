#!/bin/bash

# ========================================
# Configuration - Modify these variables
# ========================================

# Directory containing the files to process
INPUT_DIR="/mnt/t/temp/crm_get_menu_tests/"

# Output directory to write individual result files
OUTPUT_DIR="/mnt/t/temp/crm_get_menu_tests/_res"

# File pattern to match (e.g., *.txt, *.jpg, *, etc.)
FILE_PATTERN="*.json"

# Output file extension (will be appended to original filename)
OUTPUT_EXTENSION=".txt"

# Command to run on each file (use "$file" as placeholder for filename)
# Examples:
# - cat "$file"                    (display file contents)
# - ls -la "$file"                 (show file info)
# - echo "Processing: $file"       (simple echo)
# - your_program "$file"           (run your program)
COMMAND='/home/hein/hein/dev/go-http-bench/go-http-bench -config "$file"'

# ========================================
# Script execution
# ========================================

echo "Starting batch processing..."
echo "Input directory: $INPUT_DIR"
echo "Output directory: $OUTPUT_DIR"
echo "File pattern: $FILE_PATTERN"
echo "Output extension: $OUTPUT_EXTENSION"
echo "Command: $COMMAND"
echo

# Check if input directory exists
if [ ! -d "$INPUT_DIR" ]; then
    echo "Error: Input directory '$INPUT_DIR' does not exist!"
    exit 1
fi

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Create a summary log file
SUMMARY_LOG="$OUTPUT_DIR/processing_summary.log"
{
    echo "Batch processing started at $(date)"
    echo "Input directory: $INPUT_DIR"
    echo "Output directory: $OUTPUT_DIR"
    echo "File pattern: $FILE_PATTERN"
    echo "Command: $COMMAND"
    echo "========================================"
} > "$SUMMARY_LOG"

# Counter for processed files
count=0
success_count=0
error_count=0

# Change to input directory
cd "$INPUT_DIR" || exit 1

# Loop through all files matching the pattern
for file in $FILE_PATTERN; do
    # Skip if no files match the pattern
    [ ! -e "$file" ] && echo "No files found matching pattern: $FILE_PATTERN" && break
    
    # Skip directories (process files only)
    [ -d "$file" ] && continue
    
    # Generate output filename
    base_name=$(basename "$file")
    output_file="$OUTPUT_DIR/${base_name}${OUTPUT_EXTENSION}"
    
    echo "Processing: $file -> $output_file"
    
    # Create individual output file with header
    {
        echo "Processing: $file"
        echo "Timestamp: $(date)"
        echo "Command: $COMMAND"
        echo "=========================================="
    } > "$output_file"
    
    # Run the command and append output to individual file
    if eval "$COMMAND" >> "$output_file" 2>&1; then
        # Add success footer
        {
            echo "=========================================="
            echo "Processing completed successfully at $(date)"
        } >> "$output_file"
        
        echo "✓ Success: $file" | tee -a "$SUMMARY_LOG"
        ((success_count++))
    else
        # Add error footer
        {
            echo "=========================================="
            echo "Processing failed at $(date)"
        } >> "$output_file"
        
        echo "✗ Error: $file" | tee -a "$SUMMARY_LOG"
        ((error_count++))
    fi
    
    # Increment counter
    ((count++))
done

# Add completion message to summary
{
    echo "========================================"
    echo "Batch processing completed at $(date)"
    echo "Total files processed: $count"
    echo "Successful: $success_count"
    echo "Errors: $error_count"
    echo "========================================"
    echo
    echo "Individual output files created in: $OUTPUT_DIR"
} >> "$SUMMARY_LOG"

echo
echo "Processing complete!"
echo "Total files processed: $count"
echo "Successful: $success_count"
echo "Errors: $error_count"
echo "Individual output files created in: $OUTPUT_DIR"
echo "Summary log: $SUMMARY_LOG"