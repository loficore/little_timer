import type { FunctionalComponent } from "preact";
import { memo } from "preact/compat";
import { useMemo } from "preact/hooks";

type Segment = "a" | "b" | "c" | "d" | "e" | "f" | "g";

const DIGIT_SEGMENTS: Record<string, Segment[]> = {
  "0": ["a", "b", "c", "d", "e", "f"],
  "1": ["b", "c"],
  "2": ["a", "b", "d", "e", "g"],
  "3": ["a", "b", "c", "d", "g"],
  "4": ["b", "c", "f", "g"],
  "5": ["a", "c", "d", "f", "g"],
  "6": ["a", "c", "d", "e", "f", "g"],
  "7": ["a", "b", "c"],
  "8": ["a", "b", "c", "d", "e", "f", "g"],
  "9": ["a", "b", "c", "d", "f", "g"],
  "-": ["g"],
};

interface SevenSegmentDisplayProps {
  value: string;
  className?: string;
}

const SEGMENT_ORDER: Segment[] = ["a", "b", "c", "d", "e", "f", "g"];

interface SevenSegmentDigitProps {
  char: string;
  onSegments: Segment[];
}

const SevenSegmentDigit: FunctionalComponent<SevenSegmentDigitProps> = memo(({ char, onSegments }) => {
  if (char === ":") {
    return (
      <span className="seven-segment-char seven-segment-colon" aria-hidden="true">
        <span className="seven-segment-dot seven-segment-dot-top" />
        <span className="seven-segment-dot seven-segment-dot-bottom" />
      </span>
    );
  }

  return (
    <span className="seven-segment-char" aria-hidden="true">
      {SEGMENT_ORDER.map((segment) => (
        <span
          key={segment}
          className={`seven-segment-seg seven-segment-${segment} ${onSegments.includes(segment) ? "is-on" : ""}`}
        />
      ))}
    </span>
  );
});

export const SevenSegmentDisplay: FunctionalComponent<SevenSegmentDisplayProps> = memo(({ value, className = "" }) => {
  const digits = useMemo(() => Array.from(value), [value]);

  return (
    <span className={`seven-segment-display ${className}`} role="img" aria-label={value}>
      {digits.map((char, index) => {
        const onSegments = useMemo(
          () => DIGIT_SEGMENTS[char] ?? [],
          [char]
        );
        return (
          <SevenSegmentDigit
            key={`${char}-${index}`}
            char={char}
            onSegments={onSegments}
          />
        );
      })}
    </span>
  );
});
