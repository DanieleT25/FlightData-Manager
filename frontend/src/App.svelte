<script>
  import { userSession } from './store.js';
  import Register from './components/Register.svelte';
  import Login from './components/Login.svelte';
  import Dashboard from './components/Dashboard.svelte';
  import Profile from './components/Profile.svelte';

  let currentView = 'dashboard'; 

  function logout() {
    userSession.set({
      email: '',
      password: '',
      firstName: '',
      isAuthenticated: false
    });
    currentView = 'dashboard';
  }
</script>

<main class="container">
  <nav>
    <ul>
      <li><strong>Flight Monitor</strong></li>
      {#if $userSession.isAuthenticated}
        <li><a href="#" on:click|preventDefault={() => currentView = 'dashboard'} class={currentView === 'dashboard' ? 'secondary' : ''}>Dashboard</a></li>
        <li><a href="#" on:click|preventDefault={() => currentView = 'profile'} class={currentView === 'profile' ? 'secondary' : ''}>Profile</a></li>
      {/if}
    </ul>
    <ul>
      {#if $userSession.isAuthenticated}
        <li><small>Hello, {$userSession.firstName}!</small></li>
        <li><button class="outline" on:click={logout}>Log Out</button></li>
      {/if}
    </ul>
  </nav>

  <section id="content">
    {#if $userSession.isAuthenticated}
      {#if currentView === 'dashboard'}
        <Dashboard />
      {:else if currentView === 'profile'}
        <Profile />
      {/if}
    {:else}
      <div class="grid">
        <div>
          <Register />
        </div>
        <div>
          <Login />
        </div>
      </div>
    {/if}
  </section>
</main>