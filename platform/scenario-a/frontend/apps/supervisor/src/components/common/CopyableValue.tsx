import { Check, Copy } from "lucide-react";
import { useState } from "react";

interface CopyableValueProps {
  value: string;
  /** Number of characters to show before truncating. Omit to show the full value (wraps). */
  truncate?: number;
  className?: string;
}

/**
 * Displays a (optionally truncated) value with a copy-to-clipboard button.
 * - With `truncate`: single-line display, safe for table cells.
 * - Without `truncate`: full value with word-break, for detail panels.
 */
export function CopyableValue({ value, truncate, className = "" }: CopyableValueProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    void navigator.clipboard.writeText(value).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  const isTruncated = truncate !== undefined && value.length > truncate;
  const display = isTruncated ? `${value.slice(0, truncate)}…` : value;

  return (
    <span className={`inline-flex items-center gap-1 font-mono text-xs ${className}`}>
      <span
        className={isTruncated ? "whitespace-nowrap" : "break-all"}
        title={value}
      >
        {display}
      </span>
      <button
        type="button"
        onClick={(e) => { e.stopPropagation(); handleCopy(); }}
        className="shrink-0 text-muted-foreground transition-colors hover:text-foreground"
        title="Copy full value"
      >
        {copied
          ? <Check className="h-3 w-3 text-green-500" />
          : <Copy className="h-3 w-3" />}
      </button>
    </span>
  );
}
