<script setup>
import { ref, onMounted } from 'vue'

const firstName = ref('')
const lastName  = ref('')
const present   = ref([])
const msg       = ref('')
const loading   = ref(false)

async function api(path, opts = {}) {
  const res = await fetch(path, opts)
  const text = await res.text()
  let data = null
  try { data = text ? JSON.parse(text) : null } catch { /* ignore */ }
  if (!res.ok) {
    const reason = data?.error || data?.message || text || 'Error'
    throw new Error(reason)
  }
  return data
}

async function loadPresent() {
  try {
    const data = await api('/api/present')
    present.value = data || []
  } catch (e) {
    msg.value = '❌ ' + e.message
  }
}

async function checkin() {
  const fn = firstName.value.trim()
  const ln = lastName.value.trim()
  if (!fn || !ln) { msg.value = '❌ Vor- und Nachname eingeben'; return }
  loading.value = true; msg.value = ''
  try {
    await api('/api/checkin', {
      method: 'POST',
      headers: { 'Content-Type':'application/json' },
      body: JSON.stringify({ FirstName: fn, LastName: ln })
    })
    msg.value = `✅ Check-in: ${fn} ${ln}`
    await loadPresent()
  } catch (e) {
    msg.value = '❌ ' + e.message
  } finally { loading.value = false }
}

async function checkout() {
  const fn = firstName.value.trim()
  const ln = lastName.value.trim()
  if (!fn || !ln) { msg.value = '❌ Vor- und Nachname eingeben'; return }
  loading.value = true; msg.value = ''
  try {
    await api('/api/checkout', {
      method: 'POST',
      headers: { 'Content-Type':'application/json' },
      body: JSON.stringify({ FirstName: fn, LastName: ln })
    })
    msg.value = `✅ Check-out: ${fn} ${ln}`
    await loadPresent()
  } catch (e) {
    msg.value = '❌ ' + e.message
  } finally { loading.value = false }
}

onMounted(() => {
  loadPresent()
  // optional: alle 10s aktualisieren
  setInterval(loadPresent, 10000)
})
</script>

<template>
  <main style="max-width:720px;margin:2rem auto;padding:1.25rem;font-family:system-ui,Segoe UI,Arial">
    <h1 style="margin:0 0 .75rem 0;">Check-in Terminal</h1>

    <div style="display:flex;gap:.5rem;flex-wrap:wrap;margin:.5rem 0 1rem">
      <input
        v-model="firstName"
        placeholder="Vorname"
        style="flex:1;min-width:160px;padding:.6rem;border:1px solid #ccc;border-radius:.5rem"
      />
      <input
        v-model="lastName"
        placeholder="Nachname"
        style="flex:1;min-width:160px;padding:.6rem;border:1px solid #ccc;border-radius:.5rem"
      />
      <button @click="checkin" :disabled="loading"
        style="padding:.6rem 1rem;border:none;border-radius:.6rem;cursor:pointer;background:#0ea5e9;color:white">
        Check-in
      </button>
      <button @click="checkout" :disabled="loading"
        style="padding:.6rem 1rem;border:none;border-radius:.6rem;cursor:pointer;background:#10b981;color:white">
        Check-out
      </button>
    </div>

    <p v-if="msg" :style="{color: msg.startsWith('❌') ? '#dc2626' : '#065f46', margin:'0 0 1rem'}">{{ msg }}</p>

    <section>
      <h3 style="margin:.5rem 0;">Anwesend</h3>
      <div v-if="present.length === 0" style="color:#555">Niemand anwesend.</div>
      <ul v-else style="list-style:none;padding:0;margin:.5rem 0 0">
        <li v-for="p in present" :key="p.first_name + p.last_name + p.checkin_at"
            style="display:flex;justify-content:space-between;gap:1rem;padding:.6rem .75rem;border:1px solid #eee;border-radius:.6rem;margin:.4rem 0">
          <span>{{ p.first_name }} {{ p.last_name }}</span>
          <small style="color:#555">seit {{ p.checkin_at }}</small>
        </li>
      </ul>
    </section>
  </main>
</template>
