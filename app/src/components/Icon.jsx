const paths = {
  dashboard: "M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z",
  box: "M3 7l9-4 9 4v10l-9 4-9-4z M3 7l9 4 9-4 M12 11v10 M7 5l10 4",
  chart: "M4 3v18h17 M8 16v-5 M13 16V7 M18 16v-9",
  users: "M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2 M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8 M22 21v-2a4 4 0 0 0-3-3.87 M16 3.13a4 4 0 0 1 0 7.75",
  truck: "M1 4h14v12H1z M15 8h4l3 4v4h-7 M5.5 20a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5 M18.5 20a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5",
  wallet: "M3 6V4h16v4 M3 6h17v14H3z M16 11h5v5h-5z",
  settings: "M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M12 2v3 M12 19v3 M2 12h3 M19 12h3 M5 5l2 2 M17 17l2 2 M5 19l2-2 M17 7l2-2",
  logout: "M9 5H3v14h6 M9 12h12 M16 7l5 5-5 5",
  arrow: "M5 12h14 M13 6l6 6-6 6",
  menu: "M4 6h16 M4 12h16 M4 18h16",
  close: "M6 6l12 12 M6 18L18 6",
  search: "M11 3a8 8 0 1 0 0 16 8 8 0 0 0 0-16 M17 17l5 5",
  eye: "M2 12s4-7 10-7 10 7 10 7-4 7-10 7-10-7-10-7z M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6",
  check: "M5 12l4 4L19 6",
  refresh: "M20 7a9 9 0 1 0 1 8 M20 3v5h-5",
  store: "M3 10V7l2-4h14l2 4v3 M3 10h18 M5 10v11h14V10 M9 21v-7h6v7",
  receipt: "M5 3h14v19l-3-2-4 2-4-2-3 2z M8 7h8 M8 11h8 M8 15h5",
  plus: "M12 5v14 M5 12h14",
  edit: "M16 3l5 5 M4 20l4-1 12-12a2.8 2.8 0 0 0-4-4L4 15z M4 15l5 5",
  discount: "M8 16l8-8 M7 6a1 1 0 1 0 0 2 1 1 0 0 0 0-2 M17 16a1 1 0 1 0 0 2 1 1 0 0 0 0-2 M12 2l8 4v12l-8 4-8-4V6z",
  trash: "M3 6h18 M9 6V3h6v3 M5 6l1 15h12l1-15 M10 10v7 M14 10v7",
  clock: "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18 M12 7v5l3 2",
  cart: "M3 4h2l2 12h10l3-8H6 M9 21a1 1 0 1 0 0-2 1 1 0 0 0 0 2 M17 21a1 1 0 1 0 0-2 1 1 0 0 0 0 2"
};
export function Icon({
  name,
  size = 20,
  ...props
}) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" {...props}><path d={paths[name] ?? paths.box} /></svg>;
}
