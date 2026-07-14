import { useState, useEffect } from 'react';
import { useRouter } from 'next/router';
import { 
  ShieldCheck, Users, Activity, BarChart2, ShieldAlert, 
  Trash2, ArrowLeft, Ban, Play, Server, HardDrive, Cpu
} from 'lucide-react';

interface User {
  id: string;
  username: string;
  email: string;
  display_name: string;
  status: string;
  last_seen: string;
  devices?: any[];
}

export default function AdminDashboard() {
  const router = useRouter();
  const [token, setToken] = useState('');
  const [metrics, setMetrics] = useState<any>(null);
  const [users, setUsers] = useState<User[]>([]);
  const [deleteMsgId, setDeleteMsgId] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [loading, setLoading] = useState(false);

  const apiBase = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

  useEffect(() => {
    const storedToken = localStorage.getItem('ymessage_token');
    const storedUser = localStorage.getItem('ymessage_user');
    
    if (!storedToken || !storedUser) {
      router.push('/login');
      return;
    }

    const parsedUser = JSON.parse(storedUser);
    if (parsedUser.username !== 'admin') {
      router.push('/');
      return;
    }

    setToken(storedToken);
    fetchData(storedToken);
  }, []);

  const fetchData = async (authToken: string) => {
    setLoading(true);
    try {
      // 1. Fetch system metrics
      const metricsResp = await fetch(`${apiBase}/api/admin/metrics`, {
        headers: { 'Authorization': `Bearer ${authToken}` }
      });
      if (metricsResp.ok) {
        const metricsData = await metricsResp.json();
        setMetrics(metricsData);
      }

      // 2. Fetch users list
      const usersResp = await fetch(`${apiBase}/api/admin/users`, {
        headers: { 'Authorization': `Bearer ${authToken}` }
      });
      if (usersResp.ok) {
        const usersData = await usersResp.json();
        setUsers(usersData);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleBanUser = async (userId: string, isBan: boolean) => {
    setError('');
    setSuccess('');
    try {
      const response = await fetch(`${apiBase}/api/admin/users/ban`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({ user_id: userId, ban: isBan })
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error);

      setSuccess(`User status updated successfully`);
      fetchData(token);
    } catch (err: any) {
      setError(err.message || 'Failed to update user ban status');
    }
  };

  const handleDeleteMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!deleteMsgId.trim()) return;
    setError('');
    setSuccess('');

    try {
      const response = await fetch(`${apiBase}/api/admin/messages/${deleteMsgId.trim()}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error);

      setSuccess('Message deleted from system permanently');
      setDeleteMsgId('');
      fetchData(token);
    } catch (err: any) {
      setError(err.message || 'Failed to delete message');
    }
  };

  return (
    <div className="min-h-screen bg-zinc-950 p-6 text-zinc-100">
      {/* Background gradients */}
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(245,158,11,0.04),transparent_40%)]" />
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_bottom_left,rgba(0,122,255,0.04),transparent_40%)]" />

      <div className="mx-auto max-w-6xl space-y-6">
        
        {/* HEADER BAR */}
        <div className="flex items-center justify-between border-b border-zinc-800/80 pb-4">
          <div className="flex items-center gap-3">
            <button 
              onClick={() => router.push('/')} 
              className="rounded-xl border border-zinc-855 bg-zinc-900/60 p-2.5 hover:bg-zinc-850 hover:text-white transition-colors"
            >
              <ArrowLeft className="h-5 w-5" />
            </button>
            <div>
              <h1 className="text-xl font-bold text-white flex items-center gap-2">
                <ShieldCheck className="h-6 w-6 text-amber-500" />
                YMessage Orchestrator
              </h1>
              <p className="text-xs text-zinc-400">Moderation, audit logs and telemetry dashboard</p>
            </div>
          </div>

          <button
            onClick={() => fetchData(token)}
            disabled={loading}
            className="rounded-xl bg-zinc-900 border border-zinc-800 px-4 py-2 text-xs font-semibold hover:bg-zinc-850 disabled:opacity-50"
          >
            {loading ? 'Refreshing...' : 'Refresh Telemetry'}
          </button>
        </div>

        {/* METRICS ROW */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div className="glass rounded-2xl p-5 shadow-lg">
            <div className="flex items-center justify-between text-zinc-400">
              <span className="text-xs font-medium uppercase tracking-wider">Total User Accounts</span>
              <Users className="h-5 w-5 text-imessage-blue" />
            </div>
            <p className="mt-2 text-3xl font-bold text-white">{metrics?.users_total ?? '-'}</p>
          </div>

          <div className="glass rounded-2xl p-5 shadow-lg">
            <div className="flex items-center justify-between text-zinc-400">
              <span className="text-xs font-medium uppercase tracking-wider">WebSocket Sessions</span>
              <Activity className="h-5 w-5 text-emerald-400 animate-pulse" />
            </div>
            <p className="mt-2 text-3xl font-bold text-white">{metrics?.active_connections ?? '-'}</p>
          </div>

          <div className="glass rounded-2xl p-5 shadow-lg">
            <div className="flex items-center justify-between text-zinc-400">
              <span className="text-xs font-medium uppercase tracking-wider">Total Encrypted Logs</span>
              <BarChart2 className="h-5 w-5 text-purple-400" />
            </div>
            <p className="mt-2 text-3xl font-bold text-white">{metrics?.messages_total ?? '-'}</p>
          </div>

          <div className="glass rounded-2xl p-5 shadow-lg">
            <div className="flex items-center justify-between text-zinc-400">
              <span className="text-xs font-medium uppercase tracking-wider">System Heap Memory</span>
              <HardDrive className="h-5 w-5 text-amber-500" />
            </div>
            <p className="mt-2 text-3xl font-bold text-white">{metrics?.runtime_stats?.alloc_mb ? `${metrics.runtime_stats.alloc_mb} MB` : '-'}</p>
          </div>
        </div>

        {/* RUNTIME SPECS */}
        {metrics && (
          <div className="glass rounded-2xl p-5 shadow-lg">
            <h2 className="text-sm font-semibold text-white mb-3 flex items-center gap-2">
              <Server className="h-4 w-4 text-zinc-400" />
              Runtime Telemetry Spec
            </h2>
            <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
              <div className="rounded-xl bg-zinc-900/40 p-3 border border-zinc-800/40">
                <p className="text-[10px] uppercase font-bold text-zinc-500">Go Goroutines</p>
                <p className="text-lg font-semibold text-zinc-300 mt-1">{metrics.runtime_stats.goroutines}</p>
              </div>
              <div className="rounded-xl bg-zinc-900/40 p-3 border border-zinc-800/40">
                <p className="text-[10px] uppercase font-bold text-zinc-500">Total System Reserved</p>
                <p className="text-lg font-semibold text-zinc-300 mt-1">{metrics.runtime_stats.sys_mb} MB</p>
              </div>
              <div className="rounded-xl bg-zinc-900/40 p-3 border border-zinc-800/40">
                <p className="text-[10px] uppercase font-bold text-zinc-500">Garbage Collector runs</p>
                <p className="text-lg font-semibold text-zinc-300 mt-1">{metrics.runtime_stats.num_gc}</p>
              </div>
              <div className="rounded-xl bg-zinc-900/40 p-3 border border-zinc-800/40">
                <p className="text-[10px] uppercase font-bold text-zinc-500">System CPUs</p>
                <p className="text-lg font-semibold text-zinc-300 mt-1 flex items-center gap-1.5">
                  <Cpu className="h-4 w-4 text-zinc-500" />
                  Multi-core Enabled
                </p>
              </div>
            </div>
          </div>
        )}

        {/* NOTIFICATION FEEDBACKS */}
        {(error || success) && (
          <div className={`flex items-center gap-2 rounded-xl p-4 text-xs font-semibold ${error ? 'bg-red-950/40 text-red-400 border border-red-900/40' : 'bg-emerald-950/40 text-emerald-400 border border-emerald-900/40'}`}>
            <ShieldAlert className="h-4 w-4 shrink-0" />
            <span>{error || success}</span>
          </div>
        )}

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          {/* USER MANAGEMENT BOARD */}
          <div className="glass rounded-2xl p-5 shadow-lg lg:col-span-2">
            <h2 className="text-sm font-semibold text-white mb-4">User Accounts Directory</h2>
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-zinc-300">
                <thead className="border-b border-zinc-800 text-[10px] font-bold text-zinc-500 uppercase tracking-wider">
                  <tr>
                    <th className="pb-3">User Details</th>
                    <th className="pb-3">Status</th>
                    <th className="pb-3">Sessions</th>
                    <th className="pb-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-850">
                  {users.map(u => (
                    <tr key={u.id}>
                      <td className="py-3">
                        <p className="font-semibold text-white">{u.display_name}</p>
                        <p className="text-[10px] text-zinc-400">@{u.username} • {u.email}</p>
                      </td>
                      <td className="py-3">
                        <span className={`rounded-full px-2 py-0.5 text-[9px] font-bold uppercase tracking-wider ${
                          u.status === 'banned' 
                            ? 'bg-red-950/50 text-red-400 border border-red-900/40' 
                            : u.status === 'online' 
                            ? 'bg-emerald-950/50 text-emerald-400 border border-emerald-900/40'
                            : 'bg-zinc-800 text-zinc-400 border border-zinc-700/40'
                        }`}>
                          {u.status}
                        </span>
                      </td>
                      <td className="py-3 text-zinc-400">{u.devices?.length || 0} active</td>
                      <td className="py-3 text-right">
                        {u.username !== 'admin' && (
                          u.status === 'banned' ? (
                            <button
                              onClick={() => handleBanUser(u.id, false)}
                              className="rounded-lg bg-emerald-950/20 border border-emerald-900/40 px-2.5 py-1 text-[10px] font-bold text-emerald-400 hover:bg-emerald-900/30 transition-colors"
                            >
                              Activate
                            </button>
                          ) : (
                            <button
                              onClick={() => handleBanUser(u.id, true)}
                              className="rounded-lg bg-red-950/20 border border-red-900/40 px-2.5 py-1 text-[10px] font-bold text-red-400 hover:bg-red-900/30 transition-colors"
                            >
                              Suspend
                            </button>
                          )
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* MESSAGE MODERATION SIDEBAR */}
          <div className="glass rounded-2xl p-5 shadow-lg h-fit space-y-4">
            <div>
              <h2 className="text-sm font-semibold text-white flex items-center gap-1.5">
                <Trash2 className="h-4 w-4 text-red-400" />
                Moderation Actions
              </h2>
              <p className="text-[10px] text-zinc-500 mt-1">Force remove inappropriate content using specific database keys.</p>
            </div>

            <form onSubmit={handleDeleteMessage} className="space-y-3">
              <div>
                <label className="block text-[9px] uppercase font-bold text-zinc-400 mb-1">Message UUID</label>
                <input
                  type="text"
                  placeholder="e.g. 550e8400-e29b-41d4-a716-446655440000"
                  value={deleteMsgId}
                  onChange={(e) => setDeleteMsgId(e.target.value)}
                  required
                  className="w-full rounded-xl bg-zinc-900 border border-zinc-800 py-2.5 px-3 text-xs text-white placeholder-zinc-500 outline-none focus:border-red-500"
                />
              </div>

              <button
                type="submit"
                className="w-full rounded-xl bg-red-950/30 border border-red-900/50 hover:bg-red-900/30 py-2 text-xs font-semibold text-red-400 transition-colors"
              >
                Delete Content
              </button>
            </form>
          </div>
        </div>

      </div>
    </div>
  );
}
