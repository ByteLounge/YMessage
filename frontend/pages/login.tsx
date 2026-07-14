import { useState, useEffect } from 'react';
import { useRouter } from 'next/router';
import { KeyRound, Mail, User, ShieldAlert, Laptop, ArrowRight } from 'lucide-react';
import { generateKeyPair } from '@/utils/crypto';

export default function Login() {
  const router = useRouter();
  const [isLogin, setIsLogin] = useState(true);
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [deviceName, setDeviceName] = useState('Web Browser');

  useEffect(() => {
    // Auto-detect device type
    if (navigator.userAgent.includes('Windows')) setDeviceName('Windows PC (Web)');
    else if (navigator.userAgent.includes('Macintosh')) setDeviceName('Macbook (Web)');
    else if (navigator.userAgent.includes('iPhone')) setDeviceName('iPhone (Web)');
    else if (navigator.userAgent.includes('Android')) setDeviceName('Android (Web)');
    else setDeviceName('Linux PC (Web)');
  }, []);

  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    const apiBase = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

    try {
      if (isLogin) {
        // --- 1. Client-Side E2EE Key Verification/Generation ---
        let localPubKey = localStorage.getItem('ymessage_identity_public_key');
        let localPrivKey = localStorage.getItem('ymessage_identity_private_key');

        if (!localPubKey || !localPrivKey) {
          // Generate E2EE identity key pair if not exists
          const keys = await generateKeyPair();
          localStorage.setItem('ymessage_identity_public_key', keys.publicKey);
          localStorage.setItem('ymessage_identity_private_key', keys.privateKey);
          localPubKey = keys.publicKey;
        }

        // --- 2. Login to server ---
        const response = await fetch(`${apiBase}/api/auth/login`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username,
            password,
            platform: 'web',
            device_name: deviceName,
            identity_key: localPubKey,
          }),
        });

        const data = await response.json();
        if (!response.ok) throw new Error(data.error || 'Authentication failed');

        // Store tokens
        localStorage.setItem('ymessage_token', data.access_token);
        localStorage.setItem('ymessage_refresh_token', data.refresh_token);
        localStorage.setItem('ymessage_user', JSON.stringify(data.user));

        // --- 3. Register/Upload E2EE Prekeys if new session ---
        // Prepare signed prekey and one-time prekeys
        const signedPrekey = await generateKeyPair();
        localStorage.setItem('ymessage_signed_public_key', signedPrekey.publicKey);
        localStorage.setItem('ymessage_signed_private_key', signedPrekey.privateKey);

        const oneTimeKeysDto = [];
        for (let i = 0; i < 20; i++) {
          const otk = await generateKeyPair();
          oneTimeKeysDto.push({
            key_id: Math.floor(Math.random() * 1000000),
            key_val: otk.publicKey,
          });
        }

        const prekeyResp = await fetch(`${apiBase}/api/crypto/prekey`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${data.access_token}`,
          },
          body: JSON.stringify({
            identity_key: localPubKey,
            signed_prekey: signedPrekey.publicKey,
            signed_prekey_id: 1,
            signature: 'client_self_signature_placeholder', // Signature of signed prekey using Identity Key
            one_time_prekeys: oneTimeKeysDto,
          }),
        });

        if (!prekeyResp.ok) {
          const pkErr = await prekeyResp.json();
          console.warn('E2EE key registration warning:', pkErr.error);
        }

        router.push('/');
      } else {
        // --- Register ---
        const response = await fetch(`${apiBase}/api/auth/register`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username,
            email,
            password,
            display_name: displayName,
          }),
        });

        const data = await response.json();
        if (!response.ok) throw new Error(data.error || 'Registration failed');

        // Toggle back to login automatically
        setIsLogin(true);
        setError('Account created successfully! Please log in.');
      }
    } catch (err: any) {
      setError(err.message || 'An error occurred during authentication');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-950 p-4">
      {/* Background radial overlays */}
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(0,122,255,0.12),transparent_45%)]" />
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_bottom_left,rgba(52,199,89,0.08),transparent_45%)]" />

      {/* Main glassmorphic card */}
      <div className="glass w-full max-w-md rounded-2xl p-8 shadow-2xl backdrop-blur-md">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-3 flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-tr from-imessage-blue to-cyan-500 shadow-md">
            <span className="text-3xl font-bold text-white tracking-wider">Y</span>
          </div>
          <h1 className="text-2xl font-semibold tracking-tight text-white">YMessage</h1>
          <p className="mt-1 text-sm text-zinc-400">Private. Fast. Beautiful.</p>
        </div>

        {error && (
          <div className={`mb-6 flex items-center gap-2 rounded-lg p-3 text-sm ${error.includes('successful') ? 'bg-emerald-950/50 text-emerald-300 border border-emerald-800/40' : 'bg-red-950/50 text-red-300 border border-red-800/40'}`}>
            <ShieldAlert className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleAuth} className="space-y-4">
          {!isLogin && (
            <div>
              <label className="block text-xs font-medium text-zinc-400 uppercase tracking-wider mb-1">Display Name</label>
              <div className="relative">
                <input
                  type="text"
                  placeholder="John Doe"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  className="w-full rounded-xl bg-zinc-900/60 border border-zinc-800 py-3 pl-10 pr-4 text-sm text-white placeholder-zinc-500 outline-none focus:border-imessage-blue transition-all"
                />
                <User className="absolute left-3.5 top-3.5 h-4 w-4 text-zinc-500" />
              </div>
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-zinc-400 uppercase tracking-wider mb-1">Username</label>
            <div className="relative">
              <input
                type="text"
                placeholder="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                className="w-full rounded-xl bg-zinc-900/60 border border-zinc-800 py-3 pl-10 pr-4 text-sm text-white placeholder-zinc-500 outline-none focus:border-imessage-blue transition-all"
              />
              <User className="absolute left-3.5 top-3.5 h-4 w-4 text-zinc-500" />
            </div>
          </div>

          {!isLogin && (
            <div>
              <label className="block text-xs font-medium text-zinc-400 uppercase tracking-wider mb-1">Email</label>
              <div className="relative">
                <input
                  type="email"
                  placeholder="john@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  className="w-full rounded-xl bg-zinc-900/60 border border-zinc-800 py-3 pl-10 pr-4 text-sm text-white placeholder-zinc-500 outline-none focus:border-imessage-blue transition-all"
                />
                <Mail className="absolute left-3.5 top-3.5 h-4 w-4 text-zinc-500" />
              </div>
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-zinc-400 uppercase tracking-wider mb-1">Password</label>
            <div className="relative">
              <input
                type="password"
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="w-full rounded-xl bg-zinc-900/60 border border-zinc-800 py-3 pl-10 pr-4 text-sm text-white placeholder-zinc-500 outline-none focus:border-imessage-blue transition-all"
              />
              <KeyRound className="absolute left-3.5 top-3.5 h-4 w-4 text-zinc-500" />
            </div>
          </div>

          {isLogin && (
            <div className="flex items-center gap-2 rounded-xl bg-zinc-900/40 border border-zinc-800/80 p-3">
              <Laptop className="h-4 w-4 text-zinc-400 shrink-0" />
              <div className="text-xs">
                <span className="text-zinc-500">Device Session: </span>
                <span className="text-zinc-300 font-medium">{deviceName}</span>
              </div>
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="flex w-full items-center justify-center gap-2 rounded-xl bg-imessage-blue py-3 font-semibold text-white transition-all hover:bg-blue-600 disabled:opacity-50"
          >
            {loading ? 'Processing...' : isLogin ? 'Sign In' : 'Sign Up'}
            <ArrowRight className="h-4 w-4" />
          </button>
        </form>

        <div className="mt-6 text-center text-sm">
          <button
            onClick={() => {
              setIsLogin(!isLogin);
              setError('');
            }}
            className="text-zinc-400 hover:text-white transition-colors"
          >
            {isLogin ? "Don't have an account? Sign Up" : 'Already have an account? Sign In'}
          </button>
        </div>
      </div>
    </div>
  );
}
