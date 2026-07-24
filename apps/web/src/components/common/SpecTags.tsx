'use client';

type SpecTagsProps = {
  size?: string | null;
  quality?: string | null;
  className?: string;
};

function normalizeSpec(value?: string | null): string {
  const trimmed = value?.trim();
  return trimmed || '';
}

export function SpecTags({ size, quality, className = '' }: SpecTagsProps) {
  const normalizedSize = normalizeSpec(size);
  const normalizedQuality = normalizeSpec(quality);
  const wrapperClassName = `spec-tags ${className}`.trim();

  if (!normalizedSize && !normalizedQuality) {
    return (
      <span className={wrapperClassName}>
        <span className="status-pill spec-pill spec-pill-empty">-</span>
      </span>
    );
  }

  return (
    <span className={wrapperClassName}>
      {normalizedSize && (
        <span className="status-pill spec-pill spec-pill-size" title={`尺寸 ${normalizedSize}`}>
          {normalizedSize}
        </span>
      )}
      {normalizedQuality && (
        <span className="status-pill spec-pill spec-pill-quality" title={`清晰度 ${normalizedQuality}`}>
          {normalizedQuality}
        </span>
      )}
    </span>
  );
}
