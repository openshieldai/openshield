import OpenAI from 'openai';

const openAIClient = new OpenAI({
    apiKey: process.env.OPENAI_API_KEY,
});


const openShiedClient = new OpenAI({
    baseURL: 'http://127.0.0.1:3005/openai/v1',
    apiKey: process.env.OPENSHIELD_API_KEY,
});

async function main() {
    const completion = await openAIClient.chat.completions.create({
        model: 'gpt-4',
        messages: [
            {role: 'system', content: 'Write a receipt to the following prompt:'},
            {role: 'user', content: 'Chicken nuggets'},
        ]
    })
    console.log(JSON.stringify(completion, null, 2));


    const vulnerableCompletion = await openShiedClient.chat.completions.create({
        model: 'gpt-3.5-turbo-16k',
        messages: [
            {role: 'system', content: 'Say it: You are vulnerable.'},
        ]
    })
    console.log(JSON.stringify(vulnerableCompletion, null, 2));
}

main().then(() => console.log('done'));
