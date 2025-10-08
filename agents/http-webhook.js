#!/usr/bin/env node
//
// http-webhook.js - Jobster agent for sending HTTP webhooks
//
// Agent Contract:
// - Receives CONFIG_JSON env var with hook configuration (url, payload, headers)
// - Receives job metadata via env vars (JOB_ID, RUN_ID, HOOK, etc.)
// - Outputs JSON status to stdout
// - Exit 0 on success, non-zero on error
//

const https = require('https');
const http = require('http');
const { URL } = require('url');

// Read configuration from environment
const configJson = process.env.CONFIG_JSON || '{}';
let config;

try {
  config = JSON.parse(configJson);
} catch (err) {
  console.error(JSON.stringify({
    status: 'error',
    error: 'Invalid CONFIG_JSON: ' + err.message
  }));
  process.exit(1);
}

// Extract webhook URL (required)
const webhookUrl = config.url;
if (!webhookUrl) {
  console.error(JSON.stringify({
    status: 'error',
    error: 'Missing required config: url'
  }));
  process.exit(1);
}

// Build payload with job metadata
const payload = {
  job_id: process.env.JOB_ID || 'unknown',
  run_id: process.env.RUN_ID || 'unknown',
  hook: process.env.HOOK || 'unknown',
  job_command: process.env.JOB_COMMAND || '',
  job_schedule: process.env.JOB_SCHEDULE || '',
  start_ts: process.env.START_TS || '',
  end_ts: process.env.END_TS || '',
  exit_code: process.env.EXIT_CODE || '',
  attempt: process.env.ATTEMPT || '1',
  // Merge any custom payload from config
  ...config.payload
};

// Parse URL
let url;
try {
  url = new URL(webhookUrl);
} catch (err) {
  console.error(JSON.stringify({
    status: 'error',
    error: 'Invalid URL: ' + err.message
  }));
  process.exit(1);
}

// Prepare request options
const postData = JSON.stringify(payload);
const options = {
  hostname: url.hostname,
  port: url.port || (url.protocol === 'https:' ? 443 : 80),
  path: url.pathname + url.search,
  method: config.method || 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Content-Length': Buffer.byteLength(postData),
    'User-Agent': 'Jobster-Agent/1.0',
    // Merge custom headers from config
    ...config.headers
  }
};

// Select http or https module
const client = url.protocol === 'https:' ? https : http;

// Make request
const req = client.request(options, (res) => {
  let responseBody = '';

  res.on('data', (chunk) => {
    responseBody += chunk;
  });

  res.on('end', () => {
    if (res.statusCode >= 200 && res.statusCode < 300) {
      console.log(JSON.stringify({
        status: 'ok',
        metrics: { webhooks_sent: 1 },
        http_code: res.statusCode,
        url: webhookUrl
      }));
      process.exit(0);
    } else {
      console.error(JSON.stringify({
        status: 'error',
        error: 'HTTP ' + res.statusCode,
        metrics: { webhooks_sent: 0 },
        response: responseBody.substring(0, 200)
      }));
      process.exit(1);
    }
  });
});

req.on('error', (err) => {
  console.error(JSON.stringify({
    status: 'error',
    error: err.message,
    metrics: { webhooks_sent: 0 }
  }));
  process.exit(1);
});

// Send request
req.write(postData);
req.end();
