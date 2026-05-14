import type { FunctionalComponent } from "preact";

interface SkeletonProps {
  width?: string;
  height?: string;
  className?: string;
}

export const Skeleton: FunctionalComponent<SkeletonProps> = ({
  width = "100%",
  height = "1rem",
  className = "",
}) => {
  return (
    <div
      className={`skeleton rounded ${className}`}
      style={{ width, height }}
      aria-hidden="true"
    />
  );
};

interface SkeletonTextProps {
  lines?: number;
  className?: string;
}

export const SkeletonText: FunctionalComponent<SkeletonTextProps> = ({
  lines = 3,
  className = "",
}) => {
  return (
    <div className={`space-y-2 ${className}`}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton
          key={i}
          width={i === lines - 1 ? "70%" : "100%"}
          height="0.75rem"
        />
      ))}
    </div>
  );
};

interface SkeletonCardProps {
  className?: string;
}

export const SkeletonCard: FunctionalComponent<SkeletonCardProps> = ({
  className = "",
}) => {
  return (
    <div className={`p-4 rounded-lg ${className}`}>
      <Skeleton height="1.5rem" width="60%" className="mb-3" />
      <SkeletonText lines={2} />
    </div>
  );
};