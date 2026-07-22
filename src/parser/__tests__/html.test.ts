/**
 * Test script for HTML parser
 * Run with: npm run test-parser
 */
import { extractVideoUrl, findVideoUrl, isValidVideoUrl } from '../html.js';

// Test cases
const testCases = [
  {
    name: 'Video with source element',
    html: `<html><body><video width="640"><source src="https://example.com/video.mp4" type="video/mp4"></video></body></html>`,
    baseUrl: 'https://example.com/page',
    expected: 'https://example.com/video.mp4'
  },
  {
    name: 'Video with direct src',
    html: `<html><body><video src="https://example.com/movie.mp4"></video></body></html>`,
    baseUrl: 'https://example.com/page',
    expected: 'https://example.com/movie.mp4'
  },
  {
    name: 'Relative video URL',
    html: `<html><body><video><source src="/videos/clip.mp4"></video></body></html>`,
    baseUrl: 'https://example.com/page',
    expected: 'https://example.com/videos/clip.mp4'
  },
  {
    name: 'No video in HTML',
    html: `<html><body><p>No video here</p></body></html>`,
    baseUrl: 'https://example.com/page',
    expected: null
  },
  {
    name: 'Non-mp4 video (should fail)',
    html: `<html><body><video><source src="https://example.com/video.webm"></video></body></html>`,
    baseUrl: 'https://example.com/page',
    expected: null
  }
];

function runTests(): void {
  console.log('🧪 Running HTML parser tests...\n');

  let passed = 0;
  let failed = 0;

  for (const testCase of testCases) {
    const result = findVideoUrl(testCase.html, testCase.baseUrl);

    if (result === testCase.expected) {
      console.log(`✅ ${testCase.name}`);
      passed++;
    } else {
      console.log(`❌ ${testCase.name}`);
      console.log(`   Expected: ${testCase.expected}`);
      console.log(`   Got: ${result}`);
      failed++;
    }
  }

  // Test isValidVideoUrl
  console.log('\n📏 Testing isValidVideoUrl:');
  const urlTests = [
    { url: 'https://example.com/video.mp4', expected: true },
    { url: 'https://example.com/video.mp4?t=123', expected: true },
    { url: 'https://example.com/video.webm', expected: false },
    { url: '/relative/video.mp4', expected: true },
    { url: 'video.mp4', expected: true },
  ];

  for (const { url, expected } of urlTests) {
    const result = isValidVideoUrl(url);
    const status = result === expected ? '✅' : '❌';
    console.log(`   ${status} ${url} -> ${result} (expected ${expected})`);
    if (result === expected) passed++; else failed++;
  }

  console.log(`\n📊 Results: ${passed} passed, ${failed} failed`);

  if (failed > 0) {
    process.exit(1);
  }
}

runTests();