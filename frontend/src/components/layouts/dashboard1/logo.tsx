type LogoProps = {
  src?: string
  alt?: string
  onClick?: (e: React.MouseEvent<HTMLImageElement>) => void
  onPointerDown?: (e: React.PointerEvent<HTMLImageElement>) => void
}

const DEFAULT_LOGO =
  "https://shadcnuikit.com/_next/image?url=%2Flogo.png&w=64&q=75"

export function Logo({
  src = DEFAULT_LOGO,
  alt = "Logo",
  onClick,
  onPointerDown,
}: LogoProps) {
  return (
    <img
      src={src}
      width={30}
      height={30}
      className="block cursor-pointer rounded-[5px] transition-all group-data-collapsible:size-6 group-data-[collapsible=icon]:size-8"
      alt={alt}
      onClick={onClick}
      onPointerDown={onPointerDown}
    />
  )
}
