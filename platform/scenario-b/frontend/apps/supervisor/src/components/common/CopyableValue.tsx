import { Check, Copy } from "lucide-react";
import { useState } from "react";

interface CopyableValueProps {
  value: string;
  /** Number of characters to show before truncating. Omit to show full value. */
  truncate?: number;
  className?: string;
}

/**
 * Displays a (optionally truncated) value with a copy-to-clipboard button.
 * The full value is always accessible via the button and the title tooltip.
 */
export function CopyableValue({ value, truncate, className = "" }: CopyableValueProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    void navigator.clipboard.writeText(value).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  const display = truncate !== undefined && value.length > truncate
    ? `${value.slice(0, truncate)}…`
    : value;

  return (
    <span className={`inline-flex items-center gap-1 font-mono text-xs ${className}`}>
      <span className="break-all" title={value}>{display}</span>
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
