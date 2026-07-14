<script>
  import { userSession } from '../store.js';

  let confirmPassword = '';
  let loading = false;
  let errorMsg = null;

  async function handleDeleteAccount() {
    if (!confirm("Are you absolutely sure you want to permanently delete your account? This action cannot be undone.")) {
      return;
    }

    loading = true;
    errorMsg = null;

    const requestId = crypto.randomUUID();

    try {
      const response = await fetch(`/api/users/${$userSession.email}`, {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
          'X-Request-ID': requestId
        },
        body: JSON.stringify({
          password: confirmPassword
        })
      });

      if (!response.ok) {
        const errData = await response.json().catch(() => ({}));
        throw new Error(errData.detail || 'Failed to delete account. Please verify your password.');
      }

      userSession.set({
        email: '',
        password: '',
        firstName: '',
        isAuthenticated: false
      });

      alert("Account deleted successfully.");

    } catch (err) {
      errorMsg = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<article>
  <header>User Profile</header>
  
  <p><strong>First Name:</strong> {$userSession.firstName}</p>
  <p><strong>Email:</strong> {$userSession.email}</p>

  <hr />

  <details>
    <summary role="button" class="outline contrast">Delete Account</summary>
    <div style="margin-top: 1rem; padding: 1rem; background-color: var(--pico-del-color); border-radius: var(--pico-border-radius); color: white;">
      <strong>Warning:</strong> Deleting your account will permanently remove all your data.
      
      {#if errorMsg}
        <mark style="background-color: #b71c1c; color: white; display: block; margin-top: 1rem; padding: 0.5rem;">
          {errorMsg}
        </mark>
      {/if}

      <form on:submit|preventDefault={handleDeleteAccount} style="margin-top: 1rem;">
        <label style="color: white;">
          Confirm your password to proceed
          <input type="password" bind:value={confirmPassword} required style="background-color: white; color: black;" />
        </label>
        <button type="submit" class="contrast" aria-busy={loading}>Confirm Deletion</button>
      </form>
    </div>
  </details>
</article>