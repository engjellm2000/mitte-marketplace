(function () {
  const cartCount = document.querySelector('#cart-count');
  const toast = document.querySelector('#toast');
  const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content;
  let toastTimeout;

  function message(text) {
    if (!toast) return;
    toast.textContent = text;
    toast.classList.add('is-visible');
    window.clearTimeout(toastTimeout);
    toastTimeout = window.setTimeout(() => toast.classList.remove('is-visible'), 2800);
  }

  async function addToCart(button) {
    const productId = Number(button.dataset.productId);
    button.disabled = true;
    try {
      const response = await fetch('/api/cart/items', {
        method: 'POST',
        headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken},
        body: JSON.stringify({productId, quantity: 1})
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Das hat nicht funktioniert.');
      if (cartCount) {
        cartCount.textContent = payload.count;
        cartCount.setAttribute('aria-label', `${payload.count} Artikel`);
      }
      message('Zum Warenkorb hinzugefügt.');
    } catch (error) {
      message(error.message || 'Bitte versuche es erneut.');
    } finally {
      button.disabled = false;
    }
  }

  document.querySelectorAll('.add-to-cart').forEach((button) => {
    button.addEventListener('click', () => addToCart(button));
  });

  const statusPanel = document.querySelector('#order-status-panel');
  if (statusPanel && statusPanel.dataset.status === 'pending_payment') {
    const sessionId = statusPanel.dataset.sessionId;
    let attempts = 0;
    const poll = async () => {
      attempts += 1;
      try {
        const response = await fetch(`/api/orders/status?session_id=${encodeURIComponent(sessionId)}`);
        const payload = await response.json();
        if (response.ok && (payload.status === 'paid' || payload.status === 'cancelled')) {
          window.location.reload();
        }
      } catch (_) {
        // ignore transient network errors while polling
      }
      if (attempts >= 30) {
        window.clearInterval(intervalId);
      }
    };
    const intervalId = window.setInterval(poll, 2000);
    poll();
  }

  const checkout = document.querySelector('#checkout-form');
  if (checkout) {
	const checkoutIdempotencyKey = window.crypto?.randomUUID?.() || `${Date.now()}-${Math.random()}`;
    checkout.addEventListener('submit', async (event) => {
      event.preventDefault();
      const button = checkout.querySelector('button');
      button.disabled = true;
      try {
        const response = await fetch('/api/checkout', {
          method: 'POST',
		  headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken, 'Idempotency-Key': checkoutIdempotencyKey},
          body: JSON.stringify({
            name: checkout.elements.name.value,
            email: checkout.elements.email.value
          })
        });
        const payload = await response.json();
        if (!response.ok) throw new Error(payload.error || 'Bestellung fehlgeschlagen.');
        if (!payload.checkoutUrl) throw new Error('Zahlungslink fehlt.');
        window.location.href = payload.checkoutUrl;
      } catch (error) {
        message(error.message || 'Bitte versuche es erneut.');
        button.disabled = false;
      }
    });
  }
}());
