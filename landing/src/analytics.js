const fields = {
  lp_view: ['device_class'],
  lp_cta_click: ['cta_id', 'section_id'],
  lp_business_tab_select: ['business_type'],
  lp_demo_step_view: ['step'],
  lp_faq_open: ['faq_id'],
  demo_form_start: ['form_id'],
  demo_form_field_error: ['field_name', 'error_code'],
  demo_form_submit_attempt: [],
  demo_form_submit_success: ['business_type', 'branch_bucket', 'has_email'],
  demo_form_submit_error: ['error_class']
};

export function analyticsPayload(event, properties = {}) {
  if (!fields[event]) return null;
  const safe = Object.fromEntries(fields[event].filter(key => properties[key] !== undefined).map(key => [key, properties[key]]));
  return { event, properties: safe };
}

export function track(event, properties = {}) {
  const payload = analyticsPayload(event, properties);
  if (payload) window.dispatchEvent(new CustomEvent('storepos:analytics', { detail: payload }));
}
