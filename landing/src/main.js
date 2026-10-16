import { normalizePhone, validateLead, CONSENT_VERSION, PRIVACY_VERSION } from './forms.js';
import { track } from './analytics.js';
import { newIdempotencyKey } from '../../app/src/api/client.js';

document.documentElement.classList.add('js');
document.querySelectorAll('[data-year]').forEach(node => { node.textContent = new Date().getFullYear(); });
track('lp_view', { device_class: window.innerWidth < 580 ? 'mobile' : window.innerWidth < 860 ? 'tablet' : 'desktop' });

const header = document.querySelector('.site-header');
function updateHeader() { header?.classList.toggle('is-scrolled', window.scrollY > 12); }
window.addEventListener('scroll', updateHeader, { passive: true });
updateHeader();

const menu = document.querySelector('#mobile-menu');
const menuButton = document.querySelector('.menu-button');
let menuDestination;
menuButton?.addEventListener('click', () => {
  menu.showModal();
  document.body.style.overflow = 'hidden';
});
menu?.addEventListener('close', () => {
  document.body.style.overflow = '';
  if (menuDestination) { menuDestination.tabIndex = -1; menuDestination.focus({ preventScroll: true }); }
  else menuButton.focus();
  menuDestination = null;
});
menu?.querySelector('.menu-close').addEventListener('click', () => menu.close());
menu?.querySelectorAll('a').forEach(link => link.addEventListener('click', () => {
  const destination = link.getAttribute('href');
  menuDestination = destination.startsWith('#') ? document.querySelector(destination) : null;
  menu.close();
}));
window.matchMedia('(min-width: 860px)').addEventListener('change', event => { if (event.matches && menu?.open) menu.close(); });

const motionPreference = window.matchMedia('(prefers-reduced-motion: reduce)');
if ('IntersectionObserver' in window && !motionPreference.matches) {
  const observer = new IntersectionObserver(entries => {
    for (const entry of entries) entry.target.classList.toggle('is-visible', entry.isIntersecting);
  }, { threshold: 0.08, rootMargin: '0px 0px -20px 0px' });
  document.querySelectorAll('[data-reveal]').forEach(node => observer.observe(node));
  document.documentElement.classList.add('reveal-ready');
  motionPreference.addEventListener('change', event => {
    if (event.matches) { observer.disconnect(); document.documentElement.classList.remove('reveal-ready'); }
  });
}

function enableTabs(container, panels, onSelect) {
  const tabs = [...container.querySelectorAll('a')];
  container.setAttribute('role', 'tablist');
  tabs.forEach((tab, index) => {
    tab.id ||= panels[index].id + '-tab';
    tab.setAttribute('role', 'tab');
    tab.setAttribute('aria-controls', panels[index].id);
    panels[index].setAttribute('role', 'tabpanel');
    panels[index].setAttribute('aria-labelledby', tab.id);
    panels[index].tabIndex = 0;
    tab.addEventListener('click', event => { event.preventDefault(); select(index); });
    tab.addEventListener('keydown', event => {
      let next;
      if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
      if (event.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length;
      if (event.key === 'Home') next = 0;
      if (event.key === 'End') next = tabs.length - 1;
      if (next !== undefined) { event.preventDefault(); select(next); tabs[next].focus(); }
    });
  });
  function select(index, announce = true) {
    tabs.forEach((tab, i) => {
      tab.setAttribute('aria-selected', String(index === i));
      tab.tabIndex = index === i ? 0 : -1;
      panels[i].hidden = index !== i;
    });
    if (announce) onSelect(index);
  }
  select(0, false);
  return select;
}

const businessTabs = document.querySelector('.business-tabs');
if (businessTabs) enableTabs(businessTabs, [...document.querySelectorAll('[data-panel]')], index => {
  track('lp_business_tab_select', { business_type: ['retail', 'restaurant', 'pharmacy'][index] });
});
const stepTabs = document.querySelector('.demo-step-tabs');
let currentStep = 0;
if (stepTabs) {
  const previous = document.querySelector('#step-previous');
  const next = document.querySelector('#step-next');
  const names = ['Buka shift', 'Transaksi', 'Kas keluar', 'Terima barang', 'Tutup shift'];
  const selectStep = enableTabs(stepTabs, [...document.querySelectorAll('.demo-step')], index => {
    currentStep = index;
    previous.disabled = index === 0;
    next.disabled = index === names.length - 1;
    document.querySelector('#step-announcement').textContent = `Langkah ${index + 1} dari 5: ${names[index]}`;
    track('lp_demo_step_view', { step: index + 1 });
  });
  document.querySelector('.step-controls').hidden = false;
  previous.addEventListener('click', () => { selectStep(currentStep - 1); if (previous.disabled) next.focus(); });
  next.addEventListener('click', () => { selectStep(currentStep + 1); if (next.disabled) previous.focus(); });
}
document.querySelectorAll('[data-faq]').forEach(node => node.addEventListener('toggle', () => {
  if (node.open) track('lp_faq_open', { faq_id: node.dataset.faq });
}));
document.querySelectorAll('[data-cta]').forEach(node => node.addEventListener('click', () => {
  track('lp_cta_click', { cta_id: node.dataset.cta, section_id: node.closest('section')?.id || 'navigation' });
}));

const form = document.querySelector('#demo-form');
if (form) {
  const startedAt = new Date().toISOString();
  let busy = false;
  let fingerprint = '';
  let idempotencyKey;
  const button = form.querySelector('#submit-demo');
  button.disabled = false;
  form.addEventListener('focusin', () => track('demo_form_start', { form_id: 'demo' }), { once: true });
  function values() {
    const data = Object.fromEntries(new FormData(form));
    Object.keys(data).forEach(key => { if (typeof data[key] === 'string') data[key] = data[key].trim(); });
    return { ...data, consent: form.elements.consent.checked, marketing_consent: form.elements.marketing_consent.checked };
  }
  function showError(field, message) {
    const node = document.getElementById('error-' + field);
    const control = form.elements.namedItem(field);
    if (node && control) {
      node.textContent = message || '';
      control.setAttribute('aria-invalid', String(Boolean(message)));
      if (message) track('demo_form_field_error', { field_name: field, error_code: 'invalid' });
    }
  }
  form.querySelectorAll('input, select, textarea').forEach(control => {
    control.addEventListener('blur', () => showError(control.name, validateLead(values())[control.name]));
  });
  document.querySelectorAll('[data-business]').forEach(node => node.addEventListener('click', () => {
    form.elements.business_type.value = node.dataset.business;
    showError('business_type', '');
  }));
  form.addEventListener('submit', async event => {
    event.preventDefault();
    if (busy) return;
    const data = values();
    const errors = validateLead(data);
    form.querySelectorAll('.field-error').forEach(node => { node.textContent = ''; });
    form.querySelectorAll('[aria-invalid]').forEach(node => node.removeAttribute('aria-invalid'));
    const message = document.querySelector('#form-message');
    message.hidden = true;
    for (const [field, error] of Object.entries(errors)) showError(field, error);
    if (Object.keys(errors).length) { form.elements.namedItem(Object.keys(errors)[0]).focus(); return; }
    const body = { ...data, phone: normalizePhone(data.phone), started_at: startedAt, consent_version: CONSENT_VERSION, privacy_version: PRIVACY_VERSION, source_page: '/', source_tab: data.business_type };
    const serialized = JSON.stringify(body);
    if (serialized !== fingerprint) { fingerprint = serialized; idempotencyKey = newIdempotencyKey(); }
    busy = true;
    form.setAttribute('aria-busy', 'true');
    [...form.elements].forEach(control => { control.disabled = true; });
    button.textContent = 'Mengirim…';
    track('demo_form_submit_attempt');
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 20000);
    try {
      const response = await fetch('/api/v1/leads', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey }, credentials: 'omit', body: serialized, signal: controller.signal });
      const result = await response.json().catch(() => null);
      if (!response.ok || result?.accepted !== true) {
        if (response.status === 422 || response.status === 400) {
          for (const detail of result?.error?.details || []) showError(detail.field, detail.message);
        }
        const error = new Error(response.status === 429 ? 'Batas permintaan demo tercapai. Silakan coba lagi setelah satu jam.' : 'Permintaan belum terkirim. Periksa koneksi lalu coba lagi.');
        error.status = response.status;
        throw error;
      }
      form.hidden = true;
      const success = document.querySelector('#form-success');
      document.querySelector('#success-title').textContent = `Terima kasih, ${data.name.split(/\s+/)[0]}.`;
      success.hidden = false;
      success.focus();
      track('demo_form_submit_success', { business_type: data.business_type, branch_bucket: data.branches || 'unknown', has_email: Boolean(data.email) });
    } catch (error) {
      message.textContent = error.message === 'The operation was aborted.' || !error.status ? 'Permintaan belum terkirim. Periksa koneksi lalu coba lagi.' : error.message;
      message.hidden = false;
      track('demo_form_submit_error', { error_class: !error.status ? 'network' : error.status < 500 ? 'validation' : 'server' });
    } finally {
      clearTimeout(timeout);
      busy = false;
      form.removeAttribute('aria-busy');
      [...form.elements].forEach(control => { control.disabled = false; });
      button.textContent = 'Kirim permintaan demo';
      if (!form.hidden) { (form.querySelector('[aria-invalid=true]') || button).focus(); }
    }
  });
}
