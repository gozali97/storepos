// Validasi UI ganti kata sandi; backend tetap sumber kebenaran kebijakan dan daftar kata sandi umum.
export function passwordChangeBody(current, next, confirm) {
  current = String(current ?? '');
  next = String(next ?? '');
  confirm = String(confirm ?? '');
  if (!current) throw new Error('Kata sandi saat ini wajib diisi.');
  if ([...next].length < 8) throw new Error('Kata sandi baru minimal 8 karakter.');
  if (next !== confirm) throw new Error('Konfirmasi kata sandi baru tidak sama.');
  if (current === next) throw new Error('Kata sandi baru harus berbeda dari yang lama.');
  return { current_password: current, new_password: next };
}
