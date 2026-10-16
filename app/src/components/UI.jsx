import { Icon } from "./Icon.jsx";
export function PageHeading({
  title,
  description,
  children
}) {
  return <div className="page-heading">
  <div>
    <h1>{title}</h1>
    {description && <p className="muted">{description}</p>}
  </div>
  {children}
</div>;
}
export function ResourceState({
  resource,
  empty = false,
  children,
  message = "Belum ada data untuk ditampilkan."
}) {
  if (resource.loading) return <div className="empty-state" role="status"><span className="loading-ring" />Memuat data…</div>;
  if (resource.error) return <div className="empty-state">
  <Icon name="refresh" size={30} />
  <h3>{resource.error.status === 403 ? "Akses dibatasi" : "Data belum dapat dimuat"}</h3>
  <p role="alert">{resource.error.message}</p>
  <button onClick={resource.reload}>Coba lagi</button>
</div>;
  if (empty) return <div className="empty-state">
  <Icon name="box" size={32} />
  <h3>Belum ada data</h3>
  <p>{message}</p>
</div>;
  return children;
}
export function Field({
  label,
  id,
  children,
  ...props
}) {
  return <div className="field">
  <label htmlFor={id}>{label}</label>
  {children ?? <input id={id} name={id} {...props} />}
</div>;
}
export function Notice({
  children,
  kind = "error"
}) {
  return children ? <div className={`alert ${kind}`} role={kind === "error" ? "alert" : "status"}>{children}</div> : null;
}
