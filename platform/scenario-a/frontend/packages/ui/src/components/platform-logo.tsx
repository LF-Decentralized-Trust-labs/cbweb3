import { cn } from "../lib/utils";
import logoImage from "../images/logo-para-web-blancos-scaled-1-1.webp";

type PlatformLogoProps = {
  className?: string;
  imageClassName?: string;
  alt?: string;
};

export function PlatformLogo({
  className,
  imageClassName,
  alt = "LNET",
}: PlatformLogoProps) {
  return (
    <div className={cn("flex items-center", className)}>
      <img
        src={logoImage}
        alt={alt}
        className={cn("h-8 w-auto object-contain", imageClassName)}
      />
    </div>
  );
}
