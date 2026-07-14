<script>
  import { userSession } from '../store.js';

  let email = '';
  let password = '';
  
  let loading = false;
  let errorMsg = null;

  async function handleLogin() {
    loading = true;
    errorMsg = null;

    try {
      const response = await fetch('/api/interests', {
        method: 'GET',
        headers: {
          'email': email,
          'password': password
        }
      });

      if (!response.ok) {
        throw new Error('Invalid email or password.');
      }

      let firstName = 'User';
      try {
        const userRes = await fetch(`/api/users/${email}`);
        if (userRes.ok) {
          const userData = await userRes.json();
          firstName = userData.user.first_name;
        }
      } catch (e) {
        console.warn("Unable to fetch username");
      }

      userSession.set({
        email: email,
        password: password,
        firstName: firstName,
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
  <header>Login</header>
  
  {#if errorMsg}
    <mark style="background-color: #d81b60; color: white; display: block; margin-bottom: 1rem; padding: 0.5rem;">
      {errorMsg}
    </mark>
  {/if}

  <form on:submit|preventDefault={handleLogin}>
    <label>
      Email
      <input type="email" bind:value={email} required />
    </label>

    <label>
      Password
      <input type="password" bind:value={password} required />
    </label>

    <button type="submit" aria-busy={loading}>Sign In</button>
  </form>
</article>