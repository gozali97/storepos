import { test } from 'node:test';
import assert from 'node:assert/strict';
import { passwordChangeBody } from './password.js';

test('body ganti kata sandi valid dan tidak mengubah karakter non-ASCII', () => {
  assert.deepEqual(passwordChangeBody('lama-123', 'baru-päss-123', 'baru-päss-123'), {
    current_password: 'lama-123', new_password: 'baru-päss-123',
  });
});

test('ganti kata sandi menolak field kosong, pendek, konfirmasi berbeda, dan nilai sama', () => {
  assert.throws(() => passwordChangeBody('', 'baru-123', 'baru-123'));
  assert.throws(() => passwordChangeBody('lama-123', 'pendek', 'pendek'));
  assert.throws(() => passwordChangeBody('lama-123', 'baru-123', 'beda-123'));
  assert.throws(() => passwordChangeBody('lama-123', 'lama-123', 'lama-123'));
});
