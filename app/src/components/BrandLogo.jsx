const logoSources = {
  icon: "/images/icon_web_POS.png",
  header: "/images/logo_header_POS.png",
  primary: "/images/logo_utama_POS.png"
};

export function BrandLogo({ variant = "icon", className = "", alt = "Logo StorePOS" }) {
  return <img
    className={`brand-logo brand-logo--${variant} ${className}`.trim()}
    src={logoSources[variant] ?? logoSources.icon}
    alt={alt}
    decoding="async"
  />;
}
