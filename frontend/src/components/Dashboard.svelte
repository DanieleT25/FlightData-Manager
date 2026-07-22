<script>
  import { onMount } from 'svelte';
  import { userSession } from '../store.js';

  let airports = [];
  let selectedAirport = null;
  
  let lastFlight = null;
  let flightHistory = [];
  let stats = null;
  
  let loading = false;
  let errorMsg = null;

  let newAirportCode = '';
  let newHighValue = 20;
  let newLowValue = 5;
  let addLoading = false;

  const authHeaders = {
    'email': $userSession.email,
    'password': $userSession.password
  };

  onMount(async () => {
    await fetchInterests();
  });

  async function fetchInterests() {
    try {
      const response = await fetch('/api/interests', { headers: authHeaders });
      if (!response.ok) {
        const errData = await response.json().catch(() => ({}));
        throw new Error(errData.detail || `HTTP Error: ${response.status}`);
      }
      const data = await response.json();
      airports = data.tracked_airports || [];
    } catch (err) {
      errorMsg = err.message;
    }
  }

  async function addInterest() {
    addLoading = true;
    errorMsg = null;
    try {
      const payload = {
        interests: [{
            airport_code: newAirportCode.toUpperCase(),
            high_value: parseInt(newHighValue, 10),
            low_value: parseInt(newLowValue, 10)
        }]
      };
      const response = await fetch('/api/interests', {
        method: 'POST',
        headers: { ...authHeaders, 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      if (!response.ok) {
        const errData = await response.json().catch(() => ({}));
        throw new Error(errData.detail || `HTTP Error: ${response.status}`);
      }
      await fetchInterests();
      newAirportCode = ''; newHighValue = 20; newLowValue = 5;
    } catch (err) {
      errorMsg = err.message;
    } finally {
      addLoading = false;
    }
  }

  async function fetchAirportData(airportCode) {
    loading = true;
    errorMsg = null;
    selectedAirport = airportCode;
    
    lastFlight = null;
    flightHistory = [];
    stats = null;

    try {
      const [resLast, resHistory, resStats] = await Promise.all([
        fetch(`/api/airports/${airportCode}/flights/last?direction=departure`, { headers: authHeaders }),
        fetch(`/api/airports/${airportCode}/flights?direction=departure&limit=5`, { headers: authHeaders }),
        fetch(`/api/airports/${airportCode}/stats/average?direction=departure&days=7`, { headers: authHeaders })
      ]);

      if (!resLast.ok || !resHistory.ok || !resStats.ok) {
         throw new Error(`Error fetching data for ${airportCode}.`);
      }

      const dataLast = await resLast.json();
      const dataHistory = await resHistory.json();
      const dataStats = await resStats.json();

      lastFlight = dataLast.flight;
      flightHistory = dataHistory.flights || [];
      stats = dataStats;

    } catch (err) {
      errorMsg = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<div>
  {#if errorMsg}
    <mark style="background-color: #d81b60; color: white; display: block; margin-bottom: 1rem; padding: 0.5rem;">
      {errorMsg}
    </mark>
  {/if}

  <div class="grid">
    <article>
      <header>Your Airports</header>
      
      <details style="margin-bottom: 2rem;">
        <summary class="outline">Add Airport</summary>
        <form on:submit|preventDefault={addInterest} style="margin-top: 1rem;">
          <div class="grid">
            <label>
              ICAO Code
              <input type="text" bind:value={newAirportCode} placeholder="e.g. LICC" required maxlength="4" />
            </label>
          </div>
          <div class="grid">
            <label>
              High Threshold (&gt; flights)
              <input type="number" bind:value={newHighValue} required />
            </label>
            <label>
              Low Threshold (&lt; flights)
              <input type="number" bind:value={newLowValue} required />
            </label>
          </div>
          <button type="submit" aria-busy={addLoading}>Save Interest</button>
        </form>
      </details>

      {#if airports.length === 0}
        <p>No monitored airports.</p>
      {:else}
        <ul>
          {#each airports as airport}
            <li style="margin-bottom: 1rem;">
              <strong>{airport.airport_code}</strong> 
              <br>
              <small>Threshold alerts: min {airport.low_value}, max {airport.high_value}</small>
              <br>
              <button 
                class="outline" 
                style="margin-top: 0.5rem; padding: 0.25rem 0.5rem; font-size: 0.8rem;"
                on:click={() => fetchAirportData(airport.airport_code)}>
                Analyze Airport
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </article>

    <article>
      <header>
        Flight Analysis {#if selectedAirport} ({selectedAirport}) {/if}
      </header>

      {#if loading}
        <p aria-busy="true">Fetching data...</p>
      {:else if selectedAirport && stats}
        
        <div style="margin-bottom: 2rem; padding: 1rem; background-color: var(--pico-muted-border-color); border-radius: var(--pico-border-radius);">
          <h5 style="margin-bottom: 0.5rem;">Daily Average (last 7 days)</h5>
          <h2>{stats.average_daily_flights.toFixed(2)} <small style="font-size: 1rem; font-weight: normal;">flights in {stats.direction}</small></h2>
        </div>

        {#if lastFlight}
          <h5>Last Detected Flight</h5>
          <table role="grid" style="margin-bottom: 2rem;">
            <tbody>
              <tr>
                <th scope="row">Callsign</th>
                <td>{lastFlight.callsign}</td>
                <th scope="row">Time</th>
                <td>{new Date(lastFlight.firstSeen).toLocaleString()}</td>
              </tr>
            </tbody>
          </table>
        {/if}

        {#if flightHistory.length > 0}
          <h5>Recent History</h5>
          <figure>
            <table role="grid" class="striped">
              <thead>
                <tr>
                  <th>Callsign</th>
                  <th>ICAO24</th>
                  <th>Estimated Arrival</th>
                  <th>Date/Time</th>
                </tr>
              </thead>
              <tbody>
                {#each flightHistory as flight}
                  <tr>
                    <td>{flight.callsign}</td>
                    <td><code>{flight.icao24}</code></td>
                    <td>{flight.estArrivalAirport || '-'}</td>
                    <td><small>{new Date(flight.firstSeen).toLocaleString()}</small></td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </figure>
        {/if}

      {:else if selectedAirport}
        <p>Insufficient historical data for this airport.</p>
      {:else}
        <p>Select an airport from the list to start the full analysis.</p>
      {/if}
    </article>
  </div>
</div>
