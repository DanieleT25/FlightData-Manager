<script>
  import { userSession } from '../store.js';

  let formData = {
    first_name: '',
    last_name: '',
    email: '',
    password: '',
    card_number: '',
    expiration_date: '',
    cvv: ''
  };

  let loading = false;
  let errorMsg = null;
  let successMsg = null;

  async function handleRegister() {
    loading = true;
    errorMsg = null;
    successMsg = null;

    const requestId = crypto.randomUUID();

    try {
      const response = await fetch('/api/users', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Request-ID': requestId
        },
        body: JSON.stringify(formData)
      });

      if (!response.ok) {
        const errData = await response.json();
        throw new Error(errData.detail || 'Error during registration');
      }

      successMsg = 'Registration completed successfully!';
      
      userSession.set({
        email: formData.email,
        password: formData.password,
        firstName: formData.first_name,
        isAuthenticated: true
      });

    } catch (err) {
      errorMsg = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<article>
  <header>Create a new account</header>
  
  {#if errorMsg}
    <mark style="background-color: #d81b60; color: white; display: block; margin-bottom: 1rem; padding: 0.5rem;">
      {errorMsg}
    </mark>
  {/if}

  {#if successMsg}
    <mark style="background-color: #43a047; color: white; display: block; margin-bottom: 1rem; padding: 0.5rem;">
      {successMsg}
    </mark>
  {:else}
    <form on:submit|preventDefault={handleRegister}>
      
      <div class="grid">
        <label>
          First Name
          <input type="text" bind:value={formData.first_name} required />
        </label>
        <label>
          Last Name
          <input type="text" bind:value={formData.last_name} required />
        </label>
      </div>

      <label>
        Email
        <input type="email" bind:value={formData.email} required />
      </label>

      <label>
        Password
        <input type="password" bind:value={formData.password} required />
      </label>

      <fieldset>
        <legend>Credit Card Details</legend>
        <div class="grid">
          <label>
            Card Number
            <input type="text" bind:value={formData.card_number} placeholder="1234567812345678" required />
          </label>
          <label>
            Expiration Date
            <input type="text" bind:value={formData.expiration_date} placeholder="MM/YY" required />
          </label>
          <label>
            CVV
            <input type="text" bind:value={formData.cvv} placeholder="123" required />
          </label>
        </div>
      </fieldset>

      <button type="submit" aria-busy={loading}>Register</button>
    </form>
  {/if}
</article>