import { writable } from 'svelte/store';

export const userSession = writable({
    email: '',
    password: '',
    firstName: '',
    isAuthenticated: false
});