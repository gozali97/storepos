import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSession } from "../auth/SessionContext.jsx";
import { useWorkspace } from "../workspace/WorkspaceContext.jsx";
import { API_CODES, newIdempotencyKey } from "../api/client.js";
import { formatIDR, formatQty, roundToRupiah } from "../money.js";
import { rupiahToCents } from "../forms.js";
import { resolveDefaultUnit } from '../units.js';
import {
  addProduct,
  cartTotals,
  linePrice,
  reviewCart,
  isDuplicateScan,
  isScannerInput,
  discountFromInput,
  discountToInput,
  emptyCart,
  finalizeBody,
  normalizeCart,
  optionalApproval,
  quantityToMilli,
} from "../cashier.js";
import { baseQtyMilli, pharmacyErrorMessage, pharmacyFinalizeFields } from "../pharmacy.js";
import { FefoPreview } from "../components/FefoPreview.jsx";
import { Icon } from "../components/Icon.jsx";
import { Notice, PageHeading } from "../components/UI.jsx";
import { NumericKeypad, QuantityKeypad } from "../components/NumericKeypad.jsx";
import { useResource } from "../hooks/useResource.js";
import { ReceiptShare } from "../components/Receipt.jsx";
import OfflineCashier from "./OfflineCashier.jsx";

const paymentMethods = {
  cash: "Tunai",
  qris: "QRIS",
  card: "Kartu",
  transfer: "Transfer",
};

export default function CashierPage() {
  const { client, businessId, business, user } = useSession();
  const { branch, register } = useWorkspace();
  const methodSettings = useResource('/settings/payment-methods');
  const configuredMethods = methodSettings.data?.methods ?? [];
  const [drafts, setDrafts] = useState([]);
  const [active, setActive] = useState(null);
  const [catalog, setCatalog] = useState([]);
  const [results, setResults] = useState([]);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const [searching, setSearching] = useState(false);
  const [message, setMessage] = useState("");
  const [success, setSuccess] = useState("");
  const [checkout, setCheckout] = useState(false);
  const [payments, setPayments] = useState([{ method: "cash", amount: "" }]);
  const [receipt, setReceipt] = useState(null);
  const [openShift, setOpenShift] = useState(false);
  const [shiftAction, setShiftAction] = useState("");
  const [shiftResult, setShiftResult] = useState(null);
  const closedShiftSummary = useResource(shiftResult?.shift_id ? `/shifts/${shiftResult.shift_id}/summary` : null);
  const [online, setOnline] = useState(navigator.onLine);
  const [removed, setRemoved] = useState(null);
  const [pending, setPending] = useState(null);
  const [unitProduct, setUnitProduct] = useState(null);
  const [productUnits, setProductUnits] = useState({});
  const [deduplicateScans, setDeduplicateScans] = useState(true);
  const [soundEnabled, setSoundEnabled] = useState(false);
  const [scannerDetected, setScannerDetected] = useState(false);
  const [customerID, setCustomerID] = useState("");
  const [customerCursor, setCustomerCursor] = useState("");
  const [offlineMode, setOfflineMode] = useState(false);
  const [offlinePending, setOfflinePending] = useState(null);
  const [cachedSnapshot, setCachedSnapshot] = useState({});
  const [cacheReady, setCacheReady] = useState("");
  const customers = useResource(
    checkout
      ? `/customers${customerCursor ? `?after=${encodeURIComponent(customerCursor)}` : ""}`
      : null,
  );
  const openShiftPath = online && branch && register
    ? `/shifts/open?branch_id=${encodeURIComponent(branch.id)}&register_id=${encodeURIComponent(register.id)}`
    : null;
  const openShiftResource = useResource(openShiftPath);
  const activeShift = openShiftResource.data?.shift ?? null;
  useEffect(() => {
    setOpenShift(false);
  }, [branch?.id, register?.id]);
  const scanTimes = useRef([]);
  const searchInput = useRef(null);
  const discountPanel = useRef(null);
  const lastScan = useRef({ value: "", at: 0 });
  const cart = normalizeCart(active?.cart ?? emptyCart());
  const held = drafts.filter((draft) => draft.status === "held");
  const productsByID = useMemo(
    () => new Map(catalog.map((hit) => [hit.variant.id, hit])),
    [catalog],
  );
  const unitsByID = useMemo(
    () =>
      new Map(
        Object.values(productUnits)
          .flat()
          .map((unit) => [unit.unit_id, unit]),
      ),
    [productUnits],
  );
  const totals = cartTotals(cart, productsByID, unitsByID);
  const subtotal = totals?.subtotal_net_cents ?? null;
  const hasDiscount = totals && (totals.total_line_discount_cents > 0n || totals.transaction_discount_cents > 0n);
  const pendingKey = `storepos.sale.pending.${user?.id ?? "user"}.${businessId}.${register?.id ?? "register"}`;
  const cacheKey = `storepos.cashier.cache.${user?.id}.${businessId}.${branch?.id}.${register?.id}`;
  const offlineKey = `${cacheKey}.pending`;

  const mergeCatalog = useCallback((items) => {
    setCatalog((current) => {
      const merged = new Map(current.map((item) => [item.variant.id, item]));
      for (const item of items) merged.set(item.variant.id, item);
      return [...merged.values()];
    });
  }, []);

  const loadDrafts = useCallback(
    async (signal) => {
      if (!register) return;
      const data = await client.get(
        `/drafts?register_id=${encodeURIComponent(register.id)}`,
        { signal },
      );
      const items = data.drafts ?? [];
      setDrafts(items);
      setActive(items.find((item) => item.status === "active") ?? null);
    },
    [client, register?.id],
  );

  useEffect(() => {
    const controller = new AbortController();
    setDrafts([]);
    setActive(null);
    setReceipt(null);
    setMessage("");
    setCheckout(false);
    setCustomerID("");
    setCustomerCursor("");
    setOfflineMode(false);
    if (!register) return () => controller.abort();
    try {
      const snapshot = JSON.parse(localStorage.getItem(cacheKey)) || {};
      setCachedSnapshot(snapshot);
      setCatalog(snapshot.catalog ?? []);
      setResults(snapshot.catalog ?? []);
      setProductUnits(snapshot.productUnits ?? {});
      setActive(snapshot.active ?? null);
      setDrafts(snapshot.drafts ?? []);
      setOfflinePending(JSON.parse(localStorage.getItem(offlineKey)) || null);
    } catch {
      setCachedSnapshot({});
      setOfflinePending(null);
    }
    setCacheReady(cacheKey);
    Promise.all([
      loadDrafts(controller.signal),
      client
        .get("/catalog/search?limit=100", { signal: controller.signal })
        .then((data) => {
          const items = data.items ?? [];
          mergeCatalog(items);
          setResults(items);
        }),
    ]).catch((error) => {
      if (error.name !== "AbortError") setMessage(error.message);
    });
    try {
      setPending(JSON.parse(sessionStorage.getItem(pendingKey)) || null);
    } catch {
      setPending(null);
    }
    return () => controller.abort();
  }, [
    register?.id,
    businessId,
    client,
    loadDrafts,
    mergeCatalog,
    pendingKey,
    cacheKey,
    offlineKey,
  ]);

  useEffect(() => {
    if (!register || cacheReady !== cacheKey) return;
    const snapshot = { catalog, productUnits, active, drafts };
    try {
      localStorage.setItem(cacheKey, JSON.stringify(snapshot));
      setCachedSnapshot(snapshot);
    } catch {
      setMessage(
        "Cache kasir tidak dapat disimpan. Periksa ruang penyimpanan browser sebelum memakai mode offline.",
      );
    }
  }, [
    cacheKey,
    cacheReady,
    catalog,
    productUnits,
    active,
    drafts,
    register?.id,
  ]);

  useEffect(() => {
    const update = () => setOnline(navigator.onLine);
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => {
      window.removeEventListener("online", update);
      window.removeEventListener("offline", update);
    };
  }, []);

  const unitVariants = [
    ...new Set(
      cart.lines.filter((line) => line.unit_id).map((line) => line.variant_id),
    ),
  ]
    .sort()
    .join(",");
  useEffect(() => {
    const controller = new AbortController();
    for (const id of unitVariants.split(",").filter(Boolean)) {
      client
        .get(`/variants/${id}/units`, { signal: controller.signal })
        .then((data) => {
          if (!controller.signal.aborted)
            setProductUnits((current) => ({
              ...current,
              [id]: data.units ?? [],
            }));
        })
        .catch((error) => {
          if (error.name !== "AbortError") setMessage(error.message);
        });
    }
    return () => controller.abort();
  }, [unitVariants, client]);

  useEffect(() => {
    const shortcuts = (event) => {
      const editing = ["INPUT", "SELECT", "TEXTAREA"].includes(
        event.target.tagName,
      );
      if (event.key === "F2") {
        event.preventDefault();
        searchInput.current?.focus();
      }
      if (
        event.key === "F4" &&
        active?.cart?.lines?.length &&
        online &&
        !busy &&
        !pending
      ) {
        event.preventDefault();
        beginCheckout();
      }
      if (event.key === "F8" && active && !busy && !pending) {
        event.preventDefault();
        holdDraft();
      }
      if (event.key === "F6" && active) {
        event.preventDefault();
        discountPanel.current.open = true;
        discountPanel.current.querySelector("select")?.focus();
      }
      if (event.key === "Delete" && !editing && active?.cart?.lines?.length) {
        event.preventDefault();
        removeLine(active.cart.lines.length - 1);
      }
    };
    window.addEventListener("keydown", shortcuts);
    return () => window.removeEventListener("keydown", shortcuts);
  }, [active, online, busy, pending]);

  async function search(event) {
    event?.preventDefault();
    if (searching || busy || pending) return;
    const submittedAt = Date.now();
    const value = query.trim();
    if (!online) {
      const local = catalog.filter((item) =>
        `${item.product_name} ${item.variant.name} ${item.variant.sku} ${(item.variant.barcodes ?? []).join(" ")}`
          .toLowerCase()
          .includes(value.toLowerCase()),
      );
      setResults(local);
      setMessage(
        local.length ? "" : "Produk belum tersedia di cache perangkat ini.",
      );
      return;
    }
    setSearching(true);
    setMessage("");
    try {
      let barcode = null;
      if (value) {
        try {
          barcode = await client.get(
            `/catalog/barcode?barcode=${encodeURIComponent(value)}`,
          );
        } catch (error) {
          if (error.status !== 404) throw error;
        }
      }
      const data = await client.get(
        `/catalog/search?q=${encodeURIComponent(barcode?.sku ?? value)}&limit=60`,
      );
      const items = data.items ?? [];
      mergeCatalog(items);
      setResults(items);
      const exact = barcode
        ? items.find((item) => item.variant.id === barcode.variant_id)
        : items.find(
            (item) => item.variant.sku.toLowerCase() === value.toLowerCase(),
          );
      if (exact) {
        if (
          isDuplicateScan(
            lastScan.current,
            value,
            submittedAt,
            deduplicateScans,
          )
        ) {
          setMessage("Scan ganda terlalu cepat diabaikan.");
          feedback(false);
        } else {
          const unit = barcode?.unit_id
            ? {
                unit_id: barcode.unit_id,
                name: barcode.unit_name,
                factor_milli: barcode.factor_milli,
                sale_price_cents: barcode.price_cents,
              }
            : null;
          if (unit)
            setProductUnits((current) => ({
              ...current,
              [exact.variant.id]: [
                ...(current[exact.variant.id] ?? []).filter(
                  (item) => item.unit_id !== unit.unit_id,
                ),
                unit,
              ],
            }));
          if (await add(exact, unit, !barcode)) {
            lastScan.current = { value, at: submittedAt };
            setQuery("");
          }
        }
      } else if (!items.length) {
        setMessage("Produk atau barcode aktif tidak ditemukan.");
        feedback(false);
      }
    } catch (error) {
      setMessage(error.message);
      feedback(false);
    } finally {
      setSearching(false);
      searchInput.current?.focus();
    }
  }

  function feedback(ok) {
    const AudioContext = window.AudioContext ?? window.webkitAudioContext;
    if (!soundEnabled || !AudioContext) return;
    try {
      const context = new AudioContext();
      const oscillator = context.createOscillator();
      const gain = context.createGain();
      oscillator.frequency.value = ok ? 880 : 220;
      gain.gain.value = 0.08;
      oscillator.connect(gain);
      gain.connect(context.destination);
      oscillator.onended = () => {
        context.close().catch(() => {});
      };
      oscillator.start();
      oscillator.stop(context.currentTime + 0.12);
    } catch {
      /* Audio perangkat bersifat opsional; pesan visual tetap tersedia. */
    }
  }

  async function saveCart(nextCart) {
    if (!branch || !register)
      throw new Error("Pilih cabang dan terminal terlebih dahulu.");
    const saved = active
      ? await client.put(
          `/drafts/${active.id}`,
          { status: "active", cart: nextCart },
          { version: active.version },
        )
      : await client.post("/drafts", {
          branch_id: branch.id,
          register_id: register.id,
          cart: nextCart,
        });
    setActive(saved);
    setDrafts((current) => [
      ...current.filter((item) => item.id !== saved.id),
      saved,
    ]);
    return saved;
  }

  async function mutate(nextCart, successMessage = "") {
    if (busy || pending) return false;
    setBusy(true);
    setMessage("");
    setSuccess("");
    try {
      await saveCart(nextCart);
      setSuccess(successMessage);
      feedback(true);
      return true;
    } catch (error) {
      setMessage(error.message);
      feedback(false);
      return false;
    } finally {
      setBusy(false);
    }
  }

  const addingProduct = useRef(false);
  async function add(hit, unit = null, useDefault = true) {
    if (busy || pending || addingProduct.current) return;
    addingProduct.current = true;
    try {
    if (!unit && useDefault) {
      try {
        const [data, defaults] = await Promise.all([client.get(`/variants/${hit.variant.id}/units`), client.get(`/variants/${hit.variant.id}/unit-defaults`)]);
        unit = resolveDefaultUnit(data.units ?? [], defaults, 'sale');
        setProductUnits(current => ({ ...current, [hit.variant.id]: data.units ?? [] }));
      } catch (error) { setMessage(error.message); return false; }
    }
    mergeCatalog([hit]);
    const saved = await mutate(
      addProduct(cart, hit, unit),
      `${hit.product_name}${unit ? ` (${unit.name})` : ""} ditambahkan.`,
    );
    if (saved) setUnitProduct(null);
    return saved;
    } finally { addingProduct.current = false; }
  }

  async function chooseUnit(hit) {
    setMessage("");
    try {
      const data = await client.get(`/variants/${hit.variant.id}/units`);
      setProductUnits((current) => ({
        ...current,
        [hit.variant.id]: data.units ?? [],
      }));
      setUnitProduct(hit);
    } catch (error) {
      setMessage(error.message);
    }
  }

  async function changeQuantity(line, value) {
    try {
      const qty = quantityToMilli(value);
      const next = normalizeCart(cart);
      next.lines = next.lines.map((item) =>
        item === line ||
        (item.variant_id === line.variant_id && item.unit_id === line.unit_id)
          ? { ...item, qty_milli: qty }
          : item,
      );
      await mutate(next);
    } catch (error) {
      setMessage(error.message);
    }
  }

  async function saveLineOptions(event, index) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      const next = normalizeCart(cart);
      next.lines[index] = {
        ...next.lines[index],
        note: String(form.get("note")).trim(),
        discount: discountFromInput(
          form.get("discount_kind"),
          form.get("discount_value"),
        ),
      };
      await mutate(next, "Catatan dan diskon produk disimpan.");
    } catch (error) {
      setMessage(error.message);
    }
  }

  async function saveTransactionOptions(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      const next = normalizeCart(cart);
      next.note = String(form.get("note")).trim();
      next.discount = discountFromInput(
        form.get("discount_kind"),
        form.get("discount_value"),
      );
      next.discount_reason = String(form.get("discount_reason")).trim();
      await mutate(next, "Catatan dan diskon transaksi disimpan.");
    } catch (error) {
      setMessage(error.message);
    }
  }

  async function removeLine(index) {
    const next = normalizeCart(cart);
    const [line] = next.lines.splice(index, 1);
    if (!(await mutate(next))) return;
    setRemoved({ line, index, until: Date.now() + 5000 });
    setTimeout(
      () => setRemoved((current) => (current?.line === line ? null : current)),
      5000,
    );
  }

  async function undoRemove() {
    if (!removed) return;
    const next = normalizeCart(cart);
    next.lines.splice(removed.index, 0, removed.line);
    setRemoved(null);
    await mutate(next, "Produk dikembalikan ke keranjang.");
  }

  async function updateStatus(draft, status) {
    if (busy || pending) return;
    setBusy(true);
    setMessage("");
    try {
      const saved = await client.put(
        `/drafts/${draft.id}`,
        { status, cart: normalizeCart(draft.cart) },
        { version: draft.version },
      );
      await loadDrafts();
      if (status === "active") setActive(saved);
      if (status === "held" || status === "cancelled") setActive(null);
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }

  function holdDraft() {
    if (active) updateStatus(active, "held");
  }

  async function openNewShift(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setMessage("");
    try {
      await client.post("/shifts/open", {
        branch_id: branch.id,
        register_id: register.id,
        opening_cash_cents: rupiahToCents(form.get("opening_cash")),
      });
      setOpenShift(false);
      openShiftResource.reload();
      setSuccess("Shift kasir berhasil dibuka.");
    } catch (error) {
      setMessage(error.message);
      if (error.status === 409) {
        setOpenShift(false);
        openShiftResource.reload();
      }
    } finally {
      setBusy(false);
    }
  }

  async function moveCash(event) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    setBusy(true);
    setMessage("");
    setSuccess("");
    try {
      await client.post("/shifts/cash", {
        branch_id: branch.id,
        register_id: register.id,
        kind: form.get("kind"),
        amount_cents: rupiahToCents(form.get("amount")),
        reason: form.get("reason").trim(),
        approval: optionalApproval(
          form.get("approval_email"),
          form.get("approval_pin"),
        ),
      });
      formElement.reset();
      setShiftAction("");
      setSuccess("Pergerakan kas berhasil dicatat.");
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }

  async function closeCurrentShift(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setMessage("");
    setSuccess("");
    try {
      const result = await client.post("/shifts/close", {
        branch_id: branch.id,
        register_id: register.id,
        counted_cash_cents: rupiahToCents(form.get("counted_cash")),
        note: form.get("note").trim(),
        force: form.get("force") === "on",
        approval: optionalApproval(
          form.get("approval_email"),
          form.get("approval_pin"),
        ),
      });
      setShiftResult(result);
      openShiftResource.reload();
      setShiftAction("");
      setSuccess("Shift berhasil ditutup.");
    } catch (error) {
      setMessage(error.message);
    } finally {
      setBusy(false);
    }
  }

  function printShiftResult() {
    const className = "print-shift-summary";
    const cleanup = () => document.body.classList.remove(className);
    document.body.classList.add(className);
    window.addEventListener("afterprint", cleanup, { once: true });
    window.print();
  }

  function addPayment() {
    setPayments((current) => [...current, { method: "qris", amount: "" }]);
  }

  function updatePayment(index, patch) {
    setPayments((current) =>
      current.map((payment, i) =>
        i === index ? { ...payment, ...patch } : payment,
      ),
    );
  }

  async function verifyCart() {
    setBusy(true);
    setMessage("");
    try {
      const ids = [...new Set(cart.lines.map((line) => line.variant_id))];
      const loaded = await Promise.all(
        ids.map(async (id) => {
          const previous = productsByID.get(id);
          if (!previous)
            throw new Error(
              "Data produk pada draft belum termuat. Cari produk tersebut sebelum melanjutkan pembayaran.",
            );
          const items = await client.get(
            `/catalog/search?q=${encodeURIComponent(previous.variant.sku)}&limit=100`,
          );
          const product = (items.items ?? []).find(
            (item) => item.variant.id === id,
          );
          const units = cart.lines.some(
            (line) => line.variant_id === id && line.unit_id,
          )
            ? await client.get(`/variants/${id}/units`)
            : { units: [] };
          return { id, product, units: units.units ?? [] };
        }),
      );
      const latestProducts = new Map(
        loaded
          .filter((item) => item.product)
          .map((item) => [item.id, item.product]),
      );
      const latestUnits = new Map(
        loaded
          .flatMap((item) => item.units)
          .map((unit) => [unit.unit_id, unit]),
      );
      const review = reviewCart(
        cart,
        productsByID,
        unitsByID,
        latestProducts,
        latestUnits,
      );
      mergeCatalog([...latestProducts.values()]);
      setProductUnits((current) => ({
        ...current,
        ...Object.fromEntries(
          loaded
            .filter(
              (item) =>
                item.units.length ||
                cart.lines.some(
                  (line) => line.variant_id === item.id && line.unit_id,
                ),
            )
            .map((item) => [item.id, item.units]),
        ),
      }));
      if (review.unavailable.length) {
        setCheckout(false);
        setMessage(
          `Produk/satuan tidak lagi tersedia: ${review.unavailable.join(", ")}. Hapus barang tersebut dari keranjang sebelum membayar.`,
        );
        return false;
      }
      if (review.changed.length) {
        setCheckout(false);
        setMessage(
          `Harga berubah: ${review.changed.join(", ")}. Subtotal sudah diperbarui. Periksa keranjang dan nominal pembayaran, lalu tekan Bayar kembali.`,
        );
        return false;
      }
      return true;
    } catch (error) {
      setMessage(error.message);
      return false;
    } finally {
      setBusy(false);
    }
  }
  async function beginCheckout() {
    if (busy || pending || !online || !cart.lines.length) return;
    if (await verifyCart()) setCheckout(true);
  }

  async function pay(event) {
    event.preventDefault();
    if (!methodSettings.data || payments.some(p => !configuredMethods.some(m => m.code === p.method && m.enabled))) { setMessage('Pilih metode pembayaran aktif. Muat ulang pengaturan jika belum tersedia.'); methodSettings.reload(); return; }
    if (!active || !online || busy || pending) return;
    try {
      const form = new FormData(event.currentTarget);
      if (!(await verifyCart())) return;
      const body = finalizeBody(
        active,
        branch.id,
        register.id,
        payments.map((payment) => ({
          method: payment.method,
          amount_cents: rupiahToCents(payment.amount),
        })),
        customerID,
      );
      const approval = optionalApproval(
        form.get("approval_email"),
        form.get("approval_pin"),
      );
      if (approval) body.approval = approval;
      if (business?.business_type === "pharmacy") {
        Object.assign(body, pharmacyFinalizeFields(
          form.get("prescription_reference"),
          form.get("pharmacist_email"),
          form.get("pharmacist_pin"),
        ));
      }
      const operation = { key: newIdempotencyKey(), body };
      sessionStorage.setItem(pendingKey, JSON.stringify(operation));
      setPending(operation);
      await sendFinal(operation);
    } catch (error) {
      setMessage(error.message);
    }
  }

  async function sendFinal(operation) {
    setBusy(true);
    setMessage("");
    try {
      const result = await client.post("/sales/finalize", operation.body, {
        idempotencyKey: operation.key,
      });
      sessionStorage.removeItem(pendingKey);
      setPending(null);
      setReceipt(result);
      setCheckout(false);
      setPayments([{ method: "cash", amount: "" }]);
      setCustomerID("");
      setCustomerCursor("");
      setActive(null);
      await loadDrafts();
    } catch (error) {
      const uncertain =
        error.code === "NETWORK" || !error.status || error.status >= 500;
      if (!uncertain) {
        sessionStorage.removeItem(pendingKey);
        setPending(null);
      }
      if (error.code === API_CODES.SHIFT_NOT_OPEN) setOpenShift(true);
      setMessage(
        uncertain
          ? "Status transaksi belum pasti. Gunakan tombol Periksa & coba lagi dengan kunci yang sama."
          : (pharmacyErrorMessage(error.code) ?? error.message),
      );
    } finally {
      setBusy(false);
    }
  }

  if (
    register &&
    branch &&
    cacheReady === cacheKey &&
    (!online || offlineMode || offlinePending) &&
    !pending
  )
    return (
      <>
        <PageHeading
          title="Kasir offline"
          description="Susun keranjang dari produk yang sudah dimuat pada perangkat ini."
        />
        <OfflineCashier
          key={offlineKey}
          snapshot={cachedSnapshot}
          pending={offlinePending}
          storageKey={offlineKey}
          online={online}
          client={client}
          branch={branch}
          register={register}
          onPending={setOfflinePending}
          onClose={() => setOfflineMode(false)}
          onSynced={(saved) => {
            setActive(saved);
            setDrafts((current) => [
              ...current.filter((item) => item.id !== saved.id),
              saved,
            ]);
            setOfflinePending(null);
            setOfflineMode(false);
            setSuccess(
              "Keranjang offline disinkronkan. Periksa harga sebelum membayar.",
            );
          }}
        />
      </>
    );
  return (
    <>
      <PageHeading
        title="Kasir"
        description="Cari produk, susun keranjang, lalu selesaikan pembayaran pada terminal aktif."
      >
        <div className="row">
          <button
            disabled={!branch || !register || busy || !!pending}
            onClick={() => setOfflineMode(true)}
          >
            Keranjang offline
          </button>
          <button
            onClick={() => setOpenShift((value) => !value)}
            disabled={!branch || !register || busy || !online || openShiftResource.loading || !!openShiftResource.error || !!activeShift}
          >
            {activeShift ? "Shift sedang aktif" : openShiftResource.loading ? "Memeriksa shift…" : "Buka shift"}
          </button>
          <button
            onClick={() => setShiftAction((value) => (value ? "" : "cash"))}
            disabled={!branch || !register || busy}
          >
            Operasi shift
          </button>
          <button
            className="primary"
            onClick={beginCheckout}
            disabled={
              !active?.cart?.lines?.length || busy || !!pending || !online
            }
          >
            Bayar <span className="key-hint">F4</span>
          </button>
        </div>
      </PageHeading>
      <Notice>{message}</Notice>
      <Notice kind="success">{success}</Notice>
      {openShiftResource.error && online && (
        <div className="alert error" role="alert">
          Status shift belum dapat dimuat. Tombol buka shift dinonaktifkan untuk mencegah shift ganda.{" "}
          <button type="button" onClick={openShiftResource.reload}>Coba lagi</button>
        </div>
      )}
      {activeShift && branch && (
        <section className="panel active-shift" aria-labelledby="active-shift-title">
          <div>
            <span className="status-badge">Shift aktif</span>
            <h2 id="active-shift-title">Shift sedang terbuka pada {register?.name}</h2>
            <p className="muted small">Dibuka {new Date(activeShift.opened_at).toLocaleString("id-ID", { timeZone: branch.timezone })} oleh {activeShift.opened_by_current_user ? "Anda" : "anggota lain"}.</p>
          </div>
          <dl className="active-shift-cash"><dt>Kas awal di laci</dt><dd>{formatIDR(BigInt(activeShift.opening_cash_cents))}</dd></dl>
          <button type="button" onClick={() => setShiftAction("cash")} disabled={busy}>Operasi shift</button>
        </section>
      )}
      {!online && (
        <div className="alert warn" role="status">
          Mode tanpa koneksi. Pencarian memakai produk yang sudah dimuat dan
          pembayaran dinonaktifkan.
        </div>
      )}
      {pending && (
        <div className="alert warn" role="status">
          <p>Keranjang dikunci sampai status transaksi dipastikan.</p>
          <button disabled={busy} onClick={() => sendFinal(pending)}>
            {busy ? "Memeriksa…" : "Periksa & coba lagi"}
          </button>
        </div>
      )}
      {removed && (
        <div className="undo-bar" role="status">
          Produk dihapus dari keranjang.{" "}
          <button onClick={undoRemove} disabled={busy}>
            Urungkan
          </button>
        </div>
      )}

      {openShift && !activeShift && !openShiftResource.loading && !openShiftResource.error && (
        <section className="panel cashier-inline-form">
          <div>
            <h2>Buka shift terminal</h2>
            <p className="muted">
              Masukkan kas awal yang benar-benar ada di laci.
            </p>
          </div>
          <form className="row" onSubmit={openNewShift}>
            <label>
              Kas awal (Rp)
              <input
                name="opening_cash"
                type="number"
                min="0"
                step="0.01"
                defaultValue="0"
                required
              />
            </label>
            <button className="primary" disabled={busy || !branch || !register}>
              Buka shift
            </button>
          </form>
        </section>
      )}

      {shiftAction && (
        <section className="panel shift-operations">
          <div className="panel-heading">
            <div>
              <h2>Operasi shift</h2>
              <p className="muted">
                Catat kas laci atau tutup shift pada terminal {register?.name}.
              </p>
            </div>
            <button
              className="icon-button"
              aria-label="Tutup operasi shift"
              onClick={() => setShiftAction("")}
            >
              <Icon name="close" />
            </button>
          </div>
          <div className="shift-tabs">
            <button
              className={shiftAction === "cash" ? "primary" : ""}
              onClick={() => setShiftAction("cash")}
            >
              Kas masuk / keluar
            </button>
            <button
              className={shiftAction === "close" ? "primary" : ""}
              onClick={() => setShiftAction("close")}
            >
              Tutup shift
            </button>
          </div>
          {shiftAction === "cash" && (
            <form onSubmit={moveCash}>
              <fieldset disabled={busy}>
                <div className="form-grid">
                  <label className="field">
                    Jenis
                    <select name="kind" required>
                      <option value="in">Kas masuk</option>
                      <option value="out">Kas keluar</option>
                    </select>
                  </label>
                  <label className="field">
                    Nominal (Rp)
                    <input
                      name="amount"
                      type="number"
                      min="0.01"
                      step="0.01"
                      required
                    />
                  </label>
                  <label className="field">
                    Alasan
                    <input name="reason" maxLength="200" required />
                  </label>
                  <label className="field">
                    Email atasan (bila diminta)
                    <input
                      name="approval_email"
                      type="email"
                      autoComplete="username"
                    />
                  </label>
                  <label className="field">
                    PIN atasan (bila diminta)
                    <input
                      name="approval_pin"
                      type="password"
                      inputMode="numeric"
                      pattern="[0-9]{6}"
                      maxLength="6"
                      autoComplete="off"
                    />
                  </label>
                </div>
                <button className="primary">Catat pergerakan kas</button>
              </fieldset>
            </form>
          )}
          {shiftAction === "close" && (
            <form onSubmit={closeCurrentShift}>
              <fieldset disabled={busy}>
                <div className="form-grid">
                  <label className="field">
                    Tunai terhitung (Rp)
                    <input
                      name="counted_cash"
                      type="number"
                      min="0"
                      step="0.01"
                      required
                    />
                  </label>
                  <label className="field">
                    Catatan selisih
                    <input name="note" maxLength="200" />
                  </label>
                  <label className="field">
                    Email atasan (bila diminta)
                    <input
                      name="approval_email"
                      type="email"
                      autoComplete="username"
                    />
                  </label>
                  <label className="field">
                    PIN atasan (bila diminta)
                    <input
                      name="approval_pin"
                      type="password"
                      inputMode="numeric"
                      pattern="[0-9]{6}"
                      maxLength="6"
                      autoComplete="off"
                    />
                  </label>
                </div>
                {["owner", "manager"].includes(business?.role) && (
                  <label className="shift-force">
                    <input name="force" type="checkbox" /> Tutup paksa bila
                    shift dibuka pengguna lain
                  </label>
                )}
                <button className="primary">Tutup shift</button>
              </fieldset>
            </form>
          )}
        </section>
      )}

      {shiftResult && (
        <section className="panel shift-result print-shift-target" aria-live="polite">
          <h2>Ringkasan kas shift · {register?.name}</h2>
          <div>
            <span>Kas seharusnya</span>
            <strong>
              {formatIDR(BigInt(shiftResult.expected_cash_cents))}
            </strong>
          </div>
          <div>
            <span>Kas terhitung</span>
            <strong>{formatIDR(BigInt(shiftResult.counted_cash_cents))}</strong>
          </div>
          <div>
            <span>Selisih</span>
            <strong
              className={
                BigInt(shiftResult.variance_cents) === 0n
                  ? "positive"
                  : "negative"
              }
            >
              {formatIDR(BigInt(shiftResult.variance_cents))}
            </strong>
          </div>
          {closedShiftSummary.loading && <p role="status">Memuat rincian shift…</p>}
          {closedShiftSummary.error && <div className="no-print"><Notice>{closedShiftSummary.error.message}</Notice><button onClick={closedShiftSummary.reload}>Muat ulang rincian</button></div>}
          {closedShiftSummary.data && <>
            {[
              ['Kas awal', 'opening_cash_cents'], ['Kas masuk', 'cash_in_cents'], ['Kas keluar', 'cash_out_cents'],
              ['Refund tunai', 'cash_refund_cents'], ['Diskon item', 'line_discount_cents'], ['Diskon transaksi', 'transaction_discount_cents'],
            ].map(([label, key]) => <div key={key}><span>{label}</span><strong>{formatIDR(BigInt(closedShiftSummary.data[key]))}</strong></div>)}
            <div><span>Transaksi / void</span><strong>{closedShiftSummary.data.sale_count} / {closedShiftSummary.data.void_count}</strong></div>
            <h3>Pembayaran per metode</h3>
            {closedShiftSummary.data.payments.map(payment => <div key={`${payment.method}-${payment.verification}`}><span>{payment.method} · {payment.verification}</span><strong>{formatIDR(BigInt(payment.amount_cents))}</strong></div>)}
            <small>ID shift: {shiftResult.shift_id}</small>
          </>}
          <div className="row shift-result-actions no-print">
            <button onClick={printShiftResult}>Cetak ringkasan kas</button>
            <button onClick={() => setShiftResult(null)}>Tutup ringkasan</button>
          </div>
        </section>
      )}

      {receipt && (
        <section className="panel receipt-result" aria-live="polite">
          <span className="soft-icon">
            <Icon name="check" />
          </span>
          <div>
            <span>Transaksi berhasil</span>
            <h2>{receipt.receipt_number}</h2>
            <p>
              Total {formatIDR(BigInt(receipt.total_cents))} · Kembalian{" "}
              {formatIDR(BigInt(receipt.change_cents))}
            </p>
          </div>
          <button
            onClick={() => {
              setReceipt(null);
              searchInput.current?.focus();
            }}
          >
            Transaksi baru
          </button>
        </section>
      )}
      {receipt?.sale_id && (
        <section className="panel form-panel">
          <ReceiptShare key={receipt.sale_id} saleID={receipt.sale_id} />
        </section>
      )}

      {!register ? (
        <section className="panel empty-state">
          <Icon name="receipt" size={34} />
          <h2>Terminal belum dipilih</h2>
          <p>
            Pilih cabang dan terminal pada bagian atas sebelum membuka kasir.
          </p>
        </section>
      ) : (
        <div className="cashier-layout">
          <section className="panel cashier-products">
            <form className="cashier-search" onSubmit={search}>
              <Icon name="search" />
              <label className="visually-hidden" htmlFor="cashier-search">
                Cari atau scan produk
              </label>
              <input
                ref={searchInput}
                id="cashier-search"
                autoFocus
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={(event) => {
                  if (
                    event.repeat ||
                    event.ctrlKey ||
                    event.metaKey ||
                    event.altKey ||
                    event.nativeEvent.isComposing
                  )
                    return;
                  const now = performance.now();
                  if (event.key === "Enter") {
                    setScannerDetected(isScannerInput(scanTimes.current, now));
                    scanTimes.current = [];
                  } else if (event.key.length === 1) {
                    if (now - (scanTimes.current.at(-1) ?? 0) > 35)
                      scanTimes.current = [];
                    scanTimes.current.push(now);
                  } else scanTimes.current = [];
                }}
                placeholder="Scan barcode atau cari nama / SKU"
                autoComplete="off"
              />
              <button disabled={searching}>
                {searching ? "Mencari…" : "Cari"}{" "}
                <span className="key-hint">F2</span>
              </button>
            </form>
            {scannerDetected && (
              <p className="report-note" role="status">
                Scanner terdeteksi melalui input cepat + Enter.
              </p>
            )}
            <div className="checkbox-row scan-options">
              <label>
                <input
                  type="checkbox"
                  checked={deduplicateScans}
                  onChange={(event) =>
                    setDeduplicateScans(event.target.checked)
                  }
                />
                Abaikan scan ganda di bawah 300 ms
              </label>
              <label>
                <input
                  type="checkbox"
                  checked={soundEnabled}
                  onChange={(event) => setSoundEnabled(event.target.checked)}
                />
                Bunyi sukses / gagal
              </label>
            </div>
            {unitProduct && (
              <section className="unit-picker">
                <div className="panel-heading">
                  <div>
                    <h3>Pilih satuan {unitProduct.product_name}</h3>
                    <p className="muted small">
                      {unitProduct.variant.name || unitProduct.variant.sku} ·
                      Harga dan jumlah sesuai satuan yang dipilih.
                    </p>
                  </div>
                  <button onClick={() => setUnitProduct(null)}>Tutup</button>
                </div>
                <div className="row">
                  <button
                    disabled={busy || !!pending}
                    onClick={() => add(unitProduct, null, false)}
                  >
                    {unitProduct.variant.base_unit} ·{" "}
                    {formatIDR(BigInt(unitProduct.variant.price_cents))}
                  </button>
                  {(productUnits[unitProduct.variant.id] ?? []).map((unit) => (
                    <button
                      key={unit.unit_id}
                      disabled={
                        busy || !!pending || unit.sale_price_cents == null
                      }
                      onClick={() => add(unitProduct, unit)}
                    >
                      {unit.name} ·{" "}
                      {unit.sale_price_cents == null
                        ? "Harga belum diatur"
                        : formatIDR(BigInt(unit.sale_price_cents))}
                    </button>
                  ))}
                </div>
                {!productUnits[unitProduct.variant.id]?.length && (
                  <p className="muted small">
                    Produk ini hanya memiliki satuan dasar.
                  </p>
                )}
              </section>
            )}
            <div className="product-grid" aria-live="polite">
              {results.map((hit) => (
                <div className="product-choice" key={hit.variant.id}>
                  <button
                    className="product-tile"
                    disabled={busy || !!pending}
                    onClick={() => add(hit)}
                  >
                    <span className="product-avatar">
                      <Icon name="box" />
                    </span>
                    <span>
                      <strong>{hit.product_name}</strong>
                      <small>
                        {hit.variant.name || hit.variant.sku} ·{" "}
                        {hit.variant.base_unit}
                      </small>
                      {Object.keys(hit.variant.attributes ?? {}).length > 0 && (
                        <small>
                          {Object.entries(hit.variant.attributes)
                            .map(([name, value]) => `${name}: ${value}`)
                            .join(" · ")}
                        </small>
                      )}
                    </span>
                    <b>{formatIDR(BigInt(hit.variant.price_cents))}</b>
                  </button>
                  <button
                    className="unit-button"
                    disabled={busy || !!pending || !online}
                    onClick={() => chooseUnit(hit)}
                  >
                    Pilih satuan
                  </button>
                </div>
              ))}
              {!results.length && (
                <div className="empty-state">
                  <p>Tidak ada produk yang cocok.</p>
                </div>
              )}
            </div>
          </section>

          <aside className="panel cashier-cart">
            <div className="panel-heading">
              <div>
                <h2>Keranjang</h2>
                <p className="muted">{register.name}</p>
              </div>
              <span className="cart-count">{cart.lines.length}</span>
            </div>
            {held.length > 0 && (
              <details className="held-drafts">
                <summary>{held.length} transaksi ditahan</summary>
                {held.map((draft) => (
                  <div key={draft.id}>
                    <span>{draft.cart.lines.length} produk</span>
                    <span className="row">
                      <button
                        disabled={busy || !!active}
                        onClick={() => updateStatus(draft, "active")}
                      >
                        Lanjutkan
                      </button>
                      <button
                        className="danger"
                        disabled={busy}
                        onClick={() => updateStatus(draft, "cancelled")}
                      >
                        Batalkan
                      </button>
                    </span>
                  </div>
                ))}
              </details>
            )}
            {!cart.lines.length ? (
              <div className="empty-state cart-empty">
                <Icon name="receipt" size={30} />
                <h3>Keranjang masih kosong</h3>
                <p>Scan barcode atau pilih produk di sebelah kiri.</p>
              </div>
            ) : (
              <div className="cart-lines">
                {cart.lines.map((line, index) => {
                  const hit = productsByID.get(line.variant_id);
                  const unit = unitsByID.get(line.unit_id);
                  const price = linePrice(line, productsByID, unitsByID);
                  return (
                    <div
                      className="cart-line"
                      key={`${line.variant_id}-${line.unit_id}-${line.qty_milli}`}
                    >
                      <div>
                        <strong>
                          {hit?.product_name ?? "Produk pada draft"}
                        </strong>
                        <small>
                          {hit?.variant.sku ?? line.variant_id} ·{" "}
                          {line.unit_id
                            ? (unit?.name ?? "Satuan tambahan")
                            : hit?.variant.base_unit}
                          {unit
                            ? ` (${formatQty(BigInt(unit.factor_milli))} satuan dasar)`
                            : ""}
                        </small>
                        {business?.business_type === "pharmacy" && (
                          <FefoPreview
                            branchId={branch?.id}
                            variantId={line.variant_id}
                            baseQty={baseQtyMilli(line.qty_milli, unit?.factor_milli)}
                          />
                        )}
                      </div>
                      <label>
                        Qty
                        <input
                          aria-label={`Kuantitas ${hit?.product_name ?? "produk"}`}
                          type="number"
                          min="0.001"
                          step={hit?.variant.allow_fractional ? "0.001" : "1"}
                          defaultValue={formatQty(
                            BigInt(line.qty_milli),
                          ).replace(",", ".")}
                          disabled={busy || !!pending}
                          onBlur={(event) =>
                            changeQuantity(line, event.target.value)
                          }
                        />
                      </label>
                      <b>
                        {price != null
                          ? formatIDR(
                              roundToRupiah(
                                (BigInt(price) * BigInt(line.qty_milli)) /
                                  1000n,
                              ),
                            )
                          : "Harga dimuat ulang"}
                      </b>
                      <button
                        className="icon-button danger"
                        aria-label={`Hapus ${hit?.product_name ?? "produk"}`}
                        disabled={busy || !!pending}
                        onClick={() => removeLine(index)}
                      >
                        <Icon name="close" size={16} />
                      </button>
                      <QuantityKeypad
                        line={line}
                        fractional={!!hit?.variant.allow_fractional}
                        disabled={busy || !!pending}
                        onSave={(value) => changeQuantity(line, value)}
                      />
                      <details className="line-options">
                        <summary>
                          Catatan & diskon
                          {line.discount.kind !== "none" ? " · aktif" : ""}
                        </summary>
                        <form
                          onSubmit={(event) => saveLineOptions(event, index)}
                        >
                          <label>
                            Catatan produk
                            <input
                              name="note"
                              maxLength="200"
                              defaultValue={line.note}
                            />
                          </label>
                          <label>
                            Jenis diskon
                            <select
                              name="discount_kind"
                              defaultValue={line.discount.kind}
                            >
                              <option value="none">Tanpa diskon</option>
                              <option value="amount">Nominal (Rp)</option>
                              <option value="percent">Persen (%)</option>
                            </select>
                          </label>
                          <label>
                            Nilai
                            <input
                              name="discount_value"
                              type="number"
                              min="0"
                              step="0.01"
                              defaultValue={discountToInput(line.discount)}
                              placeholder="0"
                            />
                          </label>
                          <button disabled={busy || !!pending}>Terapkan</button>
                        </form>
                      </details>
                    </div>
                  );
                })}
              </div>
            )}
            <details className="transaction-options" ref={discountPanel}>
              <summary>
                Diskon & catatan transaksi <span className="key-hint">F6</span>
              </summary>
              <form onSubmit={saveTransactionOptions}>
                <label>
                  Catatan transaksi
                  <textarea
                    name="note"
                    maxLength="200"
                    defaultValue={cart.note}
                    rows="2"
                  />
                </label>
                <label>
                  Jenis diskon
                  <select
                    name="discount_kind"
                    defaultValue={cart.discount.kind}
                  >
                    <option value="none">Tanpa diskon</option>
                    <option value="amount">Nominal (Rp)</option>
                    <option value="percent">Persen (%)</option>
                  </select>
                </label>
                <label>
                  Nilai
                  <input
                    name="discount_value"
                    type="number"
                    min="0"
                    step="0.01"
                    defaultValue={discountToInput(cart.discount)}
                    placeholder="0"
                  />
                </label>
                <label>
                  Alasan diskon
                  <input
                    name="discount_reason"
                    maxLength="200"
                    defaultValue={cart.discount_reason}
                    placeholder="Wajib jika memakai approval"
                  />
                </label>
                <button disabled={!active || busy || !!pending}>Simpan</button>
              </form>
            </details>
            <dl className="cart-breakdown" aria-label="Rincian transaksi">
              <div>
                <dt>Subtotal</dt>
                <dd>{totals == null ? "—" : formatIDR(totals.subtotal_gross_cents)}</dd>
              </div>
              <div>
                <dt>Diskon</dt>
                <dd>{totals == null ? "—" : formatIDR(-(totals.total_line_discount_cents + totals.transaction_discount_cents))}</dd>
              </div>
            </dl>
            <div className="cart-total">
              <span>Total akhir (estimasi)</span>
              <strong>{subtotal == null ? "—" : formatIDR(subtotal)}</strong>
              <small>
                Total setelah diskon. Belum termasuk pajak, biaya tambahan,
                dan pembulatan saat pembayaran.
              </small>
            </div>
            <div className="cart-actions">
              <button
                disabled={!active || busy || !!pending}
                onClick={holdDraft}
              >
                Tahan <span className="key-hint">F8</span>
              </button>
              <span className="delete-hint">
                Hapus terakhir <span className="key-hint">Del</span>
              </span>
              <button
                className="primary"
                disabled={!cart.lines.length || busy || !!pending || !online}
                onClick={beginCheckout}
              >
                Bayar <span className="key-hint">F4</span>
              </button>
            </div>
          </aside>
        </div>
      )}

      {checkout && (
        <div className="modal-backdrop">
          <section
            className="panel payment-panel"
            role="dialog"
            aria-modal="true"
            aria-labelledby="payment-title"
          >
            <div className="panel-heading">
              <div>
                <h2 id="payment-title">Pembayaran</h2>
                <p className="muted">
                  {hasDiscount ? "Subtotal setelah diskon" : "Subtotal sementara"}{" "}
                  {subtotal == null ? "belum tersedia" : formatIDR(subtotal)}
                </p>
              </div>
              <button
                className="icon-button"
                aria-label="Tutup pembayaran"
                onClick={() => setCheckout(false)}
                disabled={busy || !!pending}
              >
                <Icon name="close" />
              </button>
            </div>
            <form onSubmit={pay}>
              <div className="checkout-customer">
                <label>
                  Pelanggan (opsional)
                  <select
                    value={customerID}
                    onChange={(event) => setCustomerID(event.target.value)}
                    disabled={busy || !!pending || customers.loading}
                  >
                    <option value="">Tanpa pelanggan</option>
                    {customerID &&
                      !customers.data?.customers?.some(
                        (customer) => customer.id === customerID,
                      ) && (
                        <option value={customerID}>
                          Pelanggan yang dipilih
                        </option>
                      )}
                    {customers.data?.customers?.map((customer) => (
                      <option key={customer.id} value={customer.id}>
                        {customer.name}
                        {customer.phone ? ` · ${customer.phone}` : ""}
                      </option>
                    ))}
                  </select>
                </label>
                {customers.error && (
                  <Notice>
                    {customers.error.message}{" "}
                    <button type="button" onClick={customers.reload}>
                      Muat ulang pelanggan
                    </button>
                  </Notice>
                )}
                <div className="row">
                  {customerCursor && (
                    <button
                      type="button"
                      disabled={busy || !!pending}
                      onClick={() => setCustomerCursor("")}
                    >
                      Pelanggan halaman pertama
                    </button>
                  )}
                  {customers.data?.next_cursor && (
                    <button
                      type="button"
                      disabled={busy || !!pending}
                      onClick={() =>
                        setCustomerCursor(customers.data.next_cursor)
                      }
                    >
                      Pelanggan berikutnya
                    </button>
                  )}
                </div>
              </div>
              <fieldset disabled={busy || !!pending}>
                {payments.map((payment, index) => (
                  <div className="payment-row" key={index}>
                    <label>
                      Metode
                      <select
                        value={payment.method}
                        onChange={(event) =>
                          updatePayment(index, { method: event.target.value })
                        }
                      >
                        {!configuredMethods.some(m => m.code === payment.method && m.enabled) && <option value={payment.method} disabled>Metode belum tersedia/nonaktif — pilih metode aktif</option>}
                        {configuredMethods.filter(m => m.enabled).map(m => <option key={m.code} value={m.code}>{m.name}</option>)}
                      </select>
                    </label>
                    <label>
                      Nominal (Rp)
                      <input
                        type="number"
                        min="0.01"
                        step="0.01"
                        required
                        value={payment.amount}
                        onChange={(event) =>
                          updatePayment(index, { amount: event.target.value })
                        }
                      />
                    </label>
                    {payments.length > 1 && (
                      <button
                        type="button"
                        className="danger"
                        onClick={() =>
                          setPayments((current) =>
                            current.filter((_, i) => i !== index),
                          )
                        }
                      >
                        Hapus
                      </button>
                    )}
                  </div>
                ))}
              </fieldset>
              <details className="payment-keypad">
                <summary>Keypad pembayaran</summary>
                {payments.map((payment, index) => (
                  <div key={index}>
                    <h3>
                      Pembayaran {index + 1} · {paymentMethods[payment.method]}
                    </h3>
                    <output>{payment.amount || "0"} Rupiah</output>
                    <NumericKeypad
                      value={payment.amount}
                      onChange={(amount) => updatePayment(index, { amount })}
                      disabled={busy || !!pending}
                      label={`Keypad pembayaran ${index + 1}`}
                    />
                  </div>
                ))}
              </details>
              <div className="quick-cash">
                <span>Nominal cepat</span>
                {[50000, 100000, 200000, 500000].map((value) => (
                  <button
                    type="button"
                    key={value}
                    disabled={busy || !!pending}
                    onClick={() =>
                      updatePayment(0, {
                        method: "cash",
                        amount: String(value),
                      })
                    }
                  >
                    Rp{value / 1000}rb
                  </button>
                ))}
              </div>
              {business?.business_type === "pharmacy" && (
                <details className="approval-fields" open={business?.role !== "pharmacist"}>
                  <summary>Resep dan verifikasi apoteker</summary>
                  <p className="muted small">Wajib untuk obat bertanda wajib resep. Apoteker yang sedang login terverifikasi otomatis.</p>
                  <div className="form-grid">
                    <label className="field">
                      Referensi resep
                      <input name="prescription_reference" maxLength="200" autoComplete="off" />
                    </label>
                    <label className="field">
                      Email apoteker
                      <input name="pharmacist_email" type="email" autoComplete="off" />
                    </label>
                    <label className="field">
                      PIN apoteker
                      <input name="pharmacist_pin" type="password" inputMode="numeric" pattern="[0-9]{6}" maxLength="6" autoComplete="off" />
                    </label>
                  </div>
                </details>
              )}
              <details className="approval-fields">
                <summary>Persetujuan atasan untuk diskon di atas batas</summary>
                <div className="form-grid">
                  <label className="field">
                    Email atasan
                    <input
                      name="approval_email"
                      type="email"
                      autoComplete="username"
                    />
                  </label>
                  <label className="field">
                    PIN atasan
                    <input
                      name="approval_pin"
                      type="password"
                      inputMode="numeric"
                      pattern="[0-9]{6}"
                      maxLength="6"
                      autoComplete="off"
                    />
                  </label>
                </div>
              </details>
              <div className="payment-actions">
                <button
                  type="button"
                  onClick={addPayment}
                  disabled={busy || !!pending}
                >
                  Split pembayaran
                </button>
                <button
                  className="primary"
                  disabled={busy || !!pending || !online}
                >
                  {busy ? "Memproses…" : "Selesaikan transaksi"}
                </button>
              </div>
            </form>
          </section>
        </div>
      )}
    </>
  );
}
