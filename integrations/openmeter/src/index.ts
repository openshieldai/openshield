import { RealtimeClient } from '@supabase/realtime-js';

const client = new RealtimeClient(REALTIME_URL, {
    params: {
        apikey: API_KEY
    },
})

const channel = client.channel('test-channel', {})

channel.subscribe((status, err) => {
    if (status === 'SUBSCRIBED') {
        console.log('Connected!')
    }

    if (status === 'CHANNEL_ERROR') {
        console.log(`There was an error subscribing to channel: ${err.message}`)
    }

    if (status === 'TIMED_OUT') {
        console.log('Realtime server did not respond in time.')
    }

    if (status === 'CLOSED') {
        console.log('Realtime channel was unexpectedly closed.')
    }
})