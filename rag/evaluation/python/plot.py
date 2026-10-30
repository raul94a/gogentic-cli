import argparse
import json
import sys
import re
from pathlib import Path
import matplotlib.pyplot as plt

def parse_args():
    parser = argparse.ArgumentParser(description="Plot grouped PR curves from JSON via stdin.")
    parser.add_argument(
        "--output-dir",
        "-o",
        type=str,
        required=True,
        help="Directory to save the generated charts."
    )
    return parser.parse_args()

def sanitize_filename(name):
    # Removes invalid characters for filenames
    return re.sub(r'(?u)[^-\w.]', '_', name.strip())

def main():
    args = parse_args()
    out_dir = Path(args.output_dir)
    
    # Ensure target directory exists
    out_dir.mkdir(parents=True, exist_ok=True)

    # 1. Read JSON from standard input
    try:
        input_data = json.load(sys.stdin)
    except Exception as e:
        sys.exit(f"Error reading JSON from stdin: {e}")

    # Allow processing either a single question object or a list of questions
    if isinstance(input_data, dict):
        input_data = [input_data]

    # 2. Iterate over each question
    c = 1
    for item in input_data:
        question = item.get("question", "Unknown_Question")
        results = item.get("results", [])

        if not results:
            continue

        plt.figure(figsize=(10, 7))

        # 3. Plot a curve for each overlap in the results
        for res in results:
            overlap = res.get("overlap", "Unknown")
            recall = res.get("rankedRecall", [])
            precision = res.get("rankedPrecision", [])

            if not recall or not precision:
                continue

            # Continuous lines and points
            plt.plot(
                recall,
                precision,
                linestyle="-",
                drawstyle="steps-post",
                marker="o",
                linewidth=2,
                markersize=6,
                label=f"Overlap: {overlap}"
            )
        c += 1
        # 4. Decorate the chart
        plt.xlabel("Recall", fontsize=12)
        plt.ylabel("Precision", fontsize=12)
        plt.title(f"Precision-Recall Curves\nQuestion: {str(c)} overlap {overlap}", fontsize=14, weight="bold")
        plt.xlim(0.0, 1.05)
        plt.ylim(0.0, 1.05)
        plt.grid(True, linestyle="--", alpha=0.6)
        
        # Legend to identify each overlap line
        plt.legend(loc="lower left", title="Configuration")
        plt.tight_layout()

        # 5. Save the image based on the question name
        safe_q = sanitize_filename(question)
        file_path = out_dir / f"PR_{safe_q}.png"
        
        plt.savefig(file_path, dpi=300)
        plt.close()

        print(f"Saved: {file_path.resolve()}")

if __name__ == "__main__":
    main()