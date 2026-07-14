import { useState, useEffect, useRef } from 'react';
import { useRouter } from 'next/router';
import { 
  Send, Paperclip, Smile, Search, Settings, LogOut, Check, CheckCheck, 
  MessageSquare, User as UserIcon, Shield, Sparkles, Plus, Image as ImageIcon,
  ChevronRight, Laptop, Circle, HelpCircle, RefreshCw
} from 'lucide-react';
import { deriveSharedKey, encryptMessage, decryptMessage } from '@/utils/crypto';

interface User {
  id: string;
  username: string;
  display_name: string;
  avatar_url?: string;
  bio?: string;
  status?: string;
}

interface Chat {
  chat_id: string;
  name: string;
  avatar_url?: string;
  type: string; // direct, group
  last_message?: Message;
  last_active_at: string;
  unread_count: number;
}

interface Message {
  id: string;
  sender_id: string;
  receiver_id?: string;
  group_id?: string;
  content_type: string;
  encrypted_payload: string;
  ephemeral_key?: string;
  counter?: number;
  sender_ratchet_key?: string;
  status: string; // sent, delivered, read
  created_at: string;
  decryptedText?: string;
}

export default function Home() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string>('');
  
  // Chats list and message history
  const [chats, setChats] = useState<Chat[]>([]);
  const [activeChat, setActiveChat] = useState<Chat | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputText, setInputText] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<User[]>([]);
  
  // Modals & UI States
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [devices, setDevices] = useState<any[]>([]);
  const [displayName, setDisplayName] = useState('');
  const [bio, setBio] = useState('');
  const [status, setStatus] = useState('');
  const [isTyping, setIsTyping] = useState(false);
  const [recipientTyping, setRecipientTyping] = useState<string | null>(null);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);

  // E2EE session key cache in memory
  const [sessionKeys, setSessionKeys] = useState<{ [userId: string]: CryptoKey }>({});
  
  const ws = useRef<WebSocket | null>(null);
  const messageEndRef = useRef<HTMLDivElement | null>(null);
  const apiBase = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

  useEffect(() => {
    // 1. Fetch user authentication from LocalStorage
    const storedUser = localStorage.getItem('ymessage_user');
    const storedToken = localStorage.getItem('ymessage_token');
    
    if (!storedUser || !storedToken) {
      router.push('/login');
      return;
    }
    
    const parsedUser = JSON.parse(storedUser);
    setUser(parsedUser);
    setToken(storedToken);
    setDisplayName(parsedUser.display_name || '');
    setBio(parsedUser.bio || '');
    setStatus(parsedUser.status || 'online');

    // 2. Fetch Initial Conversations
    fetchChats(storedToken);
    
    // 3. Connect to WebSocket
    connectWS(storedToken, parsedUser.id);

    return () => {
      if (ws.current) ws.current.close();
    };
  }, []);

  // Scroll to bottom of message list
  useEffect(() => {
    messageEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, recipientTyping]);

  // Fetch conversations list
  const fetchChats = async (authToken: string) => {
    try {
      const response = await fetch(`${apiBase}/api/chat/chats`, {
        headers: { 'Authorization': `Bearer ${authToken}` }
      });
      if (response.ok) {
        const data = await response.json();
        setChats(data);
      }
    } catch (err) {
      console.error('Failed to fetch chats:', err);
    }
  };

  // Connect to the real-time WebSocket hub
  const connectWS = (authToken: string, myUserId: string) => {
    const wsUrl = apiBase.replace('http', 'ws') + `/api/chat/ws`;
    const socket = new WebSocket(`${wsUrl}?access_token=${authToken}`); // WS connection
    ws.current = socket;

    socket.onopen = () => {
      console.log('WS connection established.');
    };

    socket.onmessage = async (event) => {
      const data = JSON.parse(event.data);
      
      if (data.type === 'message') {
        const rawMsg = data.payload as Message;
        
        // Decrypt the message in real-time
        const decryptedMsg = await decryptIncomingMessage(rawMsg, myUserId);
        
        // If it belongs to active chat, append it
        const isFromActiveChat = 
          (activeChat?.type === 'direct' && (rawMsg.sender_id === activeChat.chat_id || rawMsg.sender_id === myUserId)) ||
          (activeChat?.type === 'group' && rawMsg.group_id === activeChat.chat_id);
          
        if (isFromActiveChat) {
          setMessages(prev => [...prev, decryptedMsg]);
          // Send read receipt if active chat is open
          if (rawMsg.sender_id !== myUserId) {
            sendReceipt([rawMsg.id], 'read');
          }
        }
        
        // Refresh conversations list to update previews
        fetchChats(authToken);
      } else if (data.type === 'typing') {
        const payload = data.payload;
        if (activeChat && payload.chat_id === activeChat.chat_id && payload.sender_id !== myUserId) {
          setRecipientTyping(payload.is_typing ? 'typing' : null);
        }
      } else if (data.type === 'receipt') {
        const payload = data.payload;
        // Update statuses of messages
        setMessages(prev => 
          prev.map(m => payload.message_ids.includes(m.id) ? { ...m, status: payload.status } : m)
        );
      }
    };

    socket.onerror = (error) => {
      console.error('WebSocket connection error:', error);
    };

    socket.onclose = () => {
      console.log('WS connection closed. Reconnecting...');
      setTimeout(() => connectWS(authToken, myUserId), 3000);
    };
  };

  // Derive E2EE keys on-the-fly and decrypt message
  const decryptIncomingMessage = async (msg: Message, myUserId: string): Promise<Message> => {
    try {
      const partnerId = msg.sender_id === myUserId ? msg.receiver_id : msg.sender_id;
      if (!partnerId) return { ...msg, decryptedText: msg.encrypted_payload }; // Group/Plain Fallback
      
      let sharedKey = sessionKeys[partnerId];
      
      if (!sharedKey) {
        // Fetch partner's bundle
        const response = await fetch(`${apiBase}/api/crypto/prekey/${partnerId}`, {
          headers: { 'Authorization': `Bearer ${token}` }
        });
        if (!response.ok) throw new Error('Could not fetch prekey bundle');
        const bundle = await response.json();
        
        const myPrivKey = localStorage.getItem('ymessage_identity_private_key') || '';
        sharedKey = await deriveSharedKey(myPrivKey, bundle.signed_prekey);
        setSessionKeys(prev => ({ ...prev, [partnerId]: sharedKey }));
      }

      // Deconstruct ciphertext and IV from payload
      const parts = msg.encrypted_payload.split(':');
      if (parts.length !== 2) return { ...msg, decryptedText: msg.encrypted_payload };
      
      const decrypted = await decryptMessage(sharedKey, parts[0], parts[1]);
      return { ...msg, decryptedText: decrypted };
    } catch (err) {
      console.warn('Failed to decrypt message:', err);
      return { ...msg, decryptedText: '[Encrypted Payload]' };
    }
  };

  // Select a conversation and pull history
  const selectChat = async (chat: Chat) => {
    setActiveChat(chat);
    setRecipientTyping(null);
    
    try {
      const response = await fetch(`${apiBase}/api/chat/messages?chat_id=${chat.chat_id}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (response.ok) {
        const data = await response.json();
        const rawMessages = data.messages as Message[];
        
        // Decrypt all messages
        const decryptedMessages = await Promise.all(
          rawMessages.map(m => decryptIncomingMessage(m, user?.id || ''))
        );
        setMessages(decryptedMessages);

        // Mark messages as read
        const unreadIds = rawMessages
          .filter(m => m.sender_id !== user?.id && m.status !== 'read')
          .map(m => m.id);
        if (unreadIds.length > 0) {
          sendReceipt(unreadIds, 'read');
        }
      }
    } catch (err) {
      console.error('Failed to load message history:', err);
    }
  };

  // Search users to start new chat
  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!searchQuery.trim()) return;

    try {
      const response = await fetch(`${apiBase}/api/auth/users/${searchQuery}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (response.ok) {
        const data = await response.json();
        setSearchResults([data]);
      } else {
        setSearchResults([]);
      }
    } catch (err) {
      setSearchResults([]);
    }
  };

  // Open direct conversation with a searched user
  const startChatWithUser = (searchedUser: User) => {
    const newChat: Chat = {
      chat_id: searchedUser.id,
      name: searchedUser.display_name,
      avatar_url: searchedUser.avatar_url,
      type: 'direct',
      last_active_at: new Date().toISOString(),
      unread_count: 0
    };
    
    // Check if chat already exists in list
    const exists = chats.find(c => c.chat_id === searchedUser.id);
    if (!exists) {
      setChats(prev => [newChat, ...prev]);
    }
    
    setActiveChat(newChat);
    setMessages([]);
    setSearchResults([]);
    setSearchQuery('');
  };

  // Send an encrypted message
  const handleSendMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputText.trim() || !activeChat || !ws.current) return;

    const plaintext = inputText;
    setInputText('');
    
    try {
      let ciphertext = plaintext;
      let iv = '';
      
      // Only encrypt direct chats for now
      if (activeChat.type === 'direct') {
        let sharedKey = sessionKeys[activeChat.chat_id];
        
        if (!sharedKey) {
          // Fetch prekey bundle
          const response = await fetch(`${apiBase}/api/crypto/prekey/${activeChat.chat_id}`, {
            headers: { 'Authorization': `Bearer ${token}` }
          });
          if (!response.ok) throw new Error('Could not fetch prekey bundle');
          const bundle = await response.json();
          
          const myPrivKey = localStorage.getItem('ymessage_identity_private_key') || '';
          sharedKey = await deriveSharedKey(myPrivKey, bundle.signed_prekey);
          setSessionKeys(prev => ({ ...prev, [activeChat.chat_id]: sharedKey }));
        }

        const encrypted = await encryptMessage(sharedKey, plaintext);
        ciphertext = `${encrypted.ciphertext}:${encrypted.iv}`;
      }

      // Send via WebSocket
      const action = {
        type: 'message',
        payload: {
          receiver_id: activeChat.type === 'direct' ? activeChat.chat_id : null,
          group_id: activeChat.type === 'group' ? activeChat.chat_id : null,
          content_type: 'text',
          encrypted_payload: ciphertext,
          ephemeral_key: 'web-ephemeral-key',
          counter: 0,
          sender_ratchet_key: 'web-ratchet-key'
        }
      };
      
      ws.current.send(JSON.stringify(action));
      
      // Push indicator offline
      sendTypingStatus(false);
    } catch (err) {
      console.error('Failed to send encrypted message:', err);
    }
  };

  // Dispatch typing indicators
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setInputText(e.target.value);
    
    if (!isTyping) {
      setIsTyping(true);
      sendTypingStatus(true);
    }
    
    // Clear typing timeout
    const timeout = setTimeout(() => {
      setIsTyping(false);
      sendTypingStatus(false);
    }, 2000);
    return () => clearTimeout(timeout);
  };

  const sendTypingStatus = (typing: boolean) => {
    if (!activeChat || !ws.current) return;
    ws.current.send(JSON.stringify({
      type: 'typing',
      payload: {
        chat_id: activeChat.chat_id,
        is_typing: typing
      }
    }));
  };

  const sendReceipt = (messageIds: string[], status: string) => {
    if (!ws.current) return;
    ws.current.send(JSON.stringify({
      type: 'receipt',
      payload: {
        message_ids: messageIds,
        status: status
      }
    }));
  };

  // Upload file attachment
  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !activeChat) return;

    const formData = new FormData();
    formData.append('file', file);
    setUploadProgress(10);

    try {
      const response = await fetch(`${apiBase}/api/media/upload`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` },
        body: formData
      });
      setUploadProgress(70);

      if (response.ok) {
        const data = await response.json();
        setUploadProgress(100);
        
        // Send file URL as a message
        if (ws.current) {
          const action = {
            type: 'message',
            payload: {
              receiver_id: activeChat.type === 'direct' ? activeChat.chat_id : null,
              group_id: activeChat.type === 'group' ? activeChat.chat_id : null,
              content_type: file.type.startsWith('image/') ? 'photo' : 'file',
              encrypted_payload: data.file_url, // URL fits as plaintext here
              metadata: {
                file_name: data.file_name,
                thumbnail_url: data.thumbnail_url,
                file_size: data.file_size
              }
            }
          };
          ws.current.send(JSON.stringify(action));
        }
      }
    } catch (err) {
      console.error('File upload failed:', err);
    } finally {
      setTimeout(() => setUploadProgress(null), 1000);
    }
  };

  // Settings: fetch user devices
  const openSettings = async () => {
    setIsSettingsOpen(true);
    try {
      const response = await fetch(`${apiBase}/api/auth/devices`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (response.ok) {
        const data = await response.json();
        setDevices(data);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const updateProfile = async () => {
    try {
      const response = await fetch(`${apiBase}/api/auth/profile`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          display_name: displayName,
          bio: bio,
          status: status
        })
      });
      if (response.ok) {
        const updatedUser = { ...user, display_name: displayName, bio: bio, status: status } as User;
        setUser(updatedUser);
        localStorage.setItem('ymessage_user', JSON.stringify(updatedUser));
        setIsSettingsOpen(false);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleDeviceTerminate = async (devId: string) => {
    try {
      const response = await fetch(`${apiBase}/api/auth/devices/${devId}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (response.ok) {
        setDevices(prev => prev.filter(d => d.id !== devId));
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleLogout = () => {
    localStorage.clear();
    router.push('/login');
  };

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-zinc-950 p-4">
      {/* Background gradients */}
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(0,122,255,0.08),transparent_40%)]" />
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_bottom_left,rgba(52,199,89,0.04),transparent_40%)]" />

      {/* Main glass monorepo container */}
      <div className="glass flex h-full w-full overflow-hidden rounded-2xl shadow-2xl">
        
        {/* SIDEBAR */}
        <div className="flex h-full w-[350px] shrink-0 flex-col border-r border-zinc-800/80 bg-zinc-900/30">
          {/* Header */}
          <div className="flex items-center justify-between p-4 border-b border-zinc-800/50">
            <div className="flex items-center gap-3">
              <div className="relative flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-tr from-imessage-blue to-cyan-500 text-white font-bold shadow-sm">
                {user?.display_name?.charAt(0).toUpperCase() || 'U'}
                <span className="absolute -bottom-0.5 -right-0.5 h-3.5 w-3.5 rounded-full border-2 border-zinc-900 bg-emerald-500" />
              </div>
              <div>
                <h2 className="text-sm font-semibold text-white">{user?.display_name}</h2>
                <p className="text-xs text-zinc-400">@{user?.username}</p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {user?.username === 'admin' && (
                <button 
                  onClick={() => router.push('/admin')} 
                  className="rounded-lg p-2 text-zinc-400 hover:bg-zinc-800/80 hover:text-white transition-colors"
                  title="Admin Dashboard"
                >
                  <Shield className="h-5 w-5 text-amber-500" />
                </button>
              )}
              <button 
                onClick={openSettings} 
                className="rounded-lg p-2 text-zinc-400 hover:bg-zinc-800/80 hover:text-white transition-colors"
              >
                <Settings className="h-5 w-5" />
              </button>
            </div>
          </div>

          {/* Search User bar */}
          <div className="p-3">
            <form onSubmit={handleSearch} className="relative">
              <input
                type="text"
                placeholder="Search @username to chat..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full rounded-xl bg-zinc-950/80 border border-zinc-800/80 py-2.5 pl-10 pr-4 text-xs text-white placeholder-zinc-500 outline-none focus:border-imessage-blue transition-all"
              />
              <Search className="absolute left-3.5 top-3.5 h-3.5 w-3.5 text-zinc-500" />
            </form>

            {searchResults.length > 0 && (
              <div className="absolute left-8 right-8 z-10 mt-2 rounded-xl bg-zinc-900 border border-zinc-800 p-2 shadow-xl">
                {searchResults.map(resUser => (
                  <button
                    key={resUser.id}
                    onClick={() => startChatWithUser(resUser)}
                    className="flex w-full items-center gap-3 rounded-lg p-2.5 text-left hover:bg-zinc-800/50"
                  >
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-zinc-800 text-sm font-semibold">
                      {resUser.display_name.charAt(0)}
                    </div>
                    <div>
                      <p className="text-xs font-semibold text-white">{resUser.display_name}</p>
                      <p className="text-[10px] text-zinc-400">@{resUser.username}</p>
                    </div>
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Conversations list */}
          <div className="flex-1 overflow-y-auto no-scrollbar p-2 space-y-1">
            {chats.length === 0 ? (
              <div className="flex h-full flex-col items-center justify-center text-center p-6">
                <MessageSquare className="h-10 w-10 text-zinc-600 mb-3" />
                <p className="text-xs font-medium text-zinc-400">No conversations yet</p>
                <p className="text-[10px] text-zinc-500 mt-1">Search for a username above to start chatting securely.</p>
              </div>
            ) : (
              chats.map(chat => {
                const isActive = activeChat?.chat_id === chat.chat_id;
                return (
                  <button
                    key={chat.chat_id}
                    onClick={() => selectChat(chat)}
                    className={`flex w-full items-center gap-3 rounded-xl p-3 text-left transition-all ${
                      isActive ? 'bg-imessage-blue text-white shadow-md' : 'hover:bg-zinc-800/40 text-zinc-300'
                    }`}
                  >
                    <div className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-xl font-bold shadow-sm ${
                      isActive ? 'bg-white/20 text-white' : 'bg-zinc-800 text-zinc-300'
                    }`}>
                      {chat.name.charAt(0).toUpperCase()}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between">
                        <h3 className="text-xs font-semibold truncate leading-snug">{chat.name}</h3>
                        <span className={`text-[10px] ${isActive ? 'text-blue-100' : 'text-zinc-500'}`}>
                          {new Date(chat.last_active_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                        </span>
                      </div>
                      <div className="flex items-center justify-between mt-0.5">
                        <p className={`text-[11px] truncate leading-normal ${isActive ? 'text-blue-100' : 'text-zinc-400'}`}>
                          {chat.last_message?.decryptedText || 'Secure attachment'}
                        </p>
                        {chat.unread_count > 0 && !isActive && (
                          <span className="flex h-4.5 min-w-[18px] items-center justify-center rounded-full bg-emerald-500 px-1 text-[10px] font-bold text-white leading-none">
                            {chat.unread_count}
                          </span>
                        )}
                      </div>
                    </div>
                  </button>
                );
              })
            )}
          </div>
        </div>

        {/* MESSAGING PANE */}
        <div className="flex h-full flex-1 flex-col bg-zinc-950/20">
          {activeChat ? (
            <>
              {/* Header */}
              <div className="flex items-center justify-between border-b border-zinc-800/50 p-4 bg-zinc-900/10">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-zinc-800 text-white font-bold shadow-sm">
                    {activeChat.name.charAt(0).toUpperCase()}
                  </div>
                  <div>
                    <h2 className="text-sm font-semibold text-white">{activeChat.name}</h2>
                    <div className="flex items-center gap-1.5 mt-0.5">
                      <Circle className="h-1.5 w-1.5 fill-emerald-500 text-emerald-500" />
                      <span className="text-[10px] text-zinc-400">Active now</span>
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <span className="flex items-center gap-1 rounded-full bg-zinc-900 border border-zinc-800/80 px-3 py-1 text-[10px] text-zinc-400">
                    <Shield className="h-3 w-3 text-emerald-400" />
                    E2EE Secured
                  </span>
                </div>
              </div>

              {/* Message List */}
              <div className="flex-1 overflow-y-auto custom-scrollbar p-6 space-y-4">
                {messages.map((msg, index) => {
                  const isMe = msg.sender_id === user?.id;
                  const showReceipt = isMe && index === messages.length - 1;

                  return (
                    <div 
                      key={msg.id}
                      className={`flex flex-col ${isMe ? 'items-end' : 'items-start'}`}
                    >
                      <div className="flex items-end gap-1.5 max-w-[70%]">
                        <div
                          className={`rounded-2xl px-4 py-2.5 text-xs shadow-sm leading-relaxed ${
                            isMe 
                              ? 'bg-imessage-blue text-white rounded-br-sm' 
                              : 'bg-zinc-800 text-zinc-100 rounded-bl-sm'
                          }`}
                        >
                          {msg.content_type === 'photo' ? (
                            <div className="space-y-1.5">
                              <img 
                                src={msg.encrypted_payload} 
                                alt="Attachment" 
                                className="max-h-60 rounded-lg object-cover" 
                              />
                            </div>
                          ) : (
                            <p>{msg.decryptedText || msg.encrypted_payload}</p>
                          )}
                        </div>
                      </div>

                      {showReceipt && (
                        <div className="flex items-center gap-1 mt-1 text-[9px] text-zinc-500 font-semibold uppercase tracking-wider pr-1">
                          {msg.status === 'read' ? (
                            <>
                              <span className="text-imessage-blue">Read</span>
                              <CheckCheck className="h-3 w-3 text-imessage-blue" />
                            </>
                          ) : msg.status === 'delivered' ? (
                            <>
                              <span>Delivered</span>
                              <CheckCheck className="h-3 w-3" />
                            </>
                          ) : (
                            <>
                              <span>Sent</span>
                              <Check className="h-3 w-3" />
                            </>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}

                {recipientTyping && (
                  <div className="flex items-center gap-2">
                    <div className="rounded-2xl bg-zinc-850 border border-zinc-800/40 px-4 py-3 shadow-sm rounded-bl-none">
                      <div className="flex items-center gap-1">
                        <div className="h-1.5 w-1.5 animate-bounce rounded-full bg-zinc-500" style={{ animationDelay: '0ms' }} />
                        <div className="h-1.5 w-1.5 animate-bounce rounded-full bg-zinc-500" style={{ animationDelay: '150ms' }} />
                        <div className="h-1.5 w-1.5 animate-bounce rounded-full bg-zinc-500" style={{ animationDelay: '300ms' }} />
                      </div>
                    </div>
                  </div>
                )}

                <div ref={messageEndRef} />
              </div>

              {/* Uploading progress overlay */}
              {uploadProgress !== null && (
                <div className="mx-6 mb-2 flex items-center justify-between rounded-xl bg-zinc-900 border border-zinc-800 p-3">
                  <div className="flex items-center gap-2 text-xs text-zinc-400">
                    <RefreshCw className="h-3.5 w-3.5 animate-spin text-imessage-blue" />
                    <span>Uploading media attachment...</span>
                  </div>
                  <span className="text-xs text-zinc-300 font-bold">{uploadProgress}%</span>
                </div>
              )}

              {/* Input Area */}
              <div className="p-4 border-t border-zinc-800/50">
                <form onSubmit={handleSendMessage} className="flex items-center gap-3">
                  <label className="flex h-10 w-10 shrink-0 cursor-pointer items-center justify-center rounded-xl bg-zinc-900 border border-zinc-800 text-zinc-400 hover:bg-zinc-850 hover:text-white transition-colors">
                    <Paperclip className="h-5 w-5" />
                    <input 
                      type="file" 
                      onChange={handleFileUpload} 
                      className="hidden" 
                      accept="image/*,application/pdf"
                    />
                  </label>

                  <div className="relative flex-1">
                    <input
                      type="text"
                      placeholder="Type an E2EE encrypted message..."
                      value={inputText}
                      onChange={handleInputChange}
                      className="w-full rounded-xl bg-zinc-900 border border-zinc-850 py-3 pl-4 pr-10 text-xs text-white placeholder-zinc-500 outline-none focus:border-imessage-blue transition-all"
                    />
                    <button 
                      type="button" 
                      className="absolute right-3.5 top-3.5 text-zinc-500 hover:text-zinc-300"
                    >
                      <Smile className="h-4 w-4" />
                    </button>
                  </div>

                  <button
                    type="submit"
                    disabled={!inputText.trim()}
                    className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-imessage-blue text-white shadow-md transition-colors hover:bg-blue-600 disabled:opacity-50"
                  >
                    <Send className="h-4.5 w-4.5" />
                  </button>
                </form>
              </div>
            </>
          ) : (
            /* BLANK STATE */
            <div className="flex h-full flex-col items-center justify-center p-8 text-center select-none">
              <div className="relative mb-6">
                <div className="flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-tr from-imessage-blue to-cyan-500 shadow-lg shadow-blue-500/10">
                  <span className="text-4xl font-extrabold text-white tracking-widest">Y</span>
                </div>
                <div className="absolute -bottom-2 -right-2 flex h-7 w-7 items-center justify-center rounded-full bg-zinc-900 border border-zinc-800 shadow-md">
                  <Sparkles className="h-4 w-4 text-emerald-400" />
                </div>
              </div>
              <h2 className="text-xl font-bold tracking-tight text-white">YMessage Desktop</h2>
              <p className="mt-2 max-w-sm text-xs text-zinc-400 leading-relaxed">
                Welcome to the production release of YMessage. All communications are private, end-to-end encrypted locally, and completely hidden from the server.
              </p>
              <div className="mt-8 flex flex-col gap-2 rounded-xl bg-zinc-900/50 border border-zinc-800/60 p-4 max-w-xs">
                <div className="flex items-center gap-2.5 text-left text-[11px] text-zinc-400">
                  <Shield className="h-4 w-4 text-emerald-400 shrink-0" />
                  <span>X25519 Double Ratchet cryptographic sessions</span>
                </div>
                <div className="flex items-center gap-2.5 text-left text-[11px] text-zinc-400">
                  <CheckCheck className="h-4 w-4 text-imessage-blue shrink-0" />
                  <span>Real-time delivery & read synchronization</span>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* SETTINGS DRAWER / MODAL */}
      {isSettingsOpen && (
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="glass w-full max-w-lg rounded-2xl p-6 shadow-2xl animate-in fade-in zoom-in-95 duration-200">
            <div className="flex items-center justify-between border-b border-zinc-800/80 pb-3">
              <h2 className="text-base font-bold text-white flex items-center gap-2">
                <Settings className="h-5 w-5 text-imessage-blue" />
                System Settings
              </h2>
              <button 
                onClick={() => setIsSettingsOpen(false)}
                className="text-xs text-zinc-400 hover:text-white"
              >
                Close
              </button>
            </div>

            <div className="mt-4 space-y-4 max-h-[400px] overflow-y-auto no-scrollbar pr-1">
              {/* Profile setup */}
              <div>
                <h3 className="text-xs font-semibold text-zinc-300 uppercase tracking-wider mb-2">User Profile</h3>
                <div className="space-y-3 rounded-xl bg-zinc-900/40 border border-zinc-800 p-4">
                  <div>
                    <label className="block text-[10px] text-zinc-500 uppercase font-semibold mb-1">Display Name</label>
                    <input 
                      type="text" 
                      value={displayName}
                      onChange={(e) => setDisplayName(e.target.value)}
                      className="w-full rounded-lg bg-zinc-900 border border-zinc-800 py-2 px-3 text-xs text-white outline-none focus:border-imessage-blue"
                    />
                  </div>
                  <div>
                    <label className="block text-[10px] text-zinc-500 uppercase font-semibold mb-1">Bio</label>
                    <textarea 
                      value={bio}
                      onChange={(e) => setBio(e.target.value)}
                      rows={2}
                      className="w-full rounded-lg bg-zinc-900 border border-zinc-800 py-2 px-3 text-xs text-white outline-none focus:border-imessage-blue resize-none"
                    />
                  </div>
                </div>
              </div>

              {/* Devices manager */}
              <div>
                <h3 className="text-xs font-semibold text-zinc-300 uppercase tracking-wider mb-2">Connected Devices</h3>
                <div className="space-y-2">
                  {devices.map((device: any) => (
                    <div 
                      key={device.id}
                      className="flex items-center justify-between rounded-xl bg-zinc-900/40 border border-zinc-800 p-3"
                    >
                      <div className="flex items-center gap-3">
                        <Laptop className="h-5 w-5 text-zinc-400" />
                        <div>
                          <div className="flex items-center gap-1.5">
                            <h4 className="text-xs font-semibold text-white">{device.device_name}</h4>
                            <span className="rounded bg-zinc-800 px-1 text-[8px] text-zinc-500 font-bold uppercase tracking-wider">{device.platform}</span>
                          </div>
                          <p className="text-[10px] text-zinc-500">Active: {new Date(device.last_active_at).toLocaleString()}</p>
                        </div>
                      </div>
                      
                      <button
                        onClick={() => handleDeviceTerminate(device.id)}
                        className="rounded-lg border border-red-950 bg-red-950/20 px-2.5 py-1 text-[10px] font-bold text-red-400 hover:bg-red-900/30 transition-colors"
                      >
                        Revoke
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            <div className="mt-6 flex items-center justify-between border-t border-zinc-800/80 pt-4">
              <button
                onClick={handleLogout}
                className="flex items-center gap-1.5 rounded-lg border border-red-950 bg-red-950/20 px-4 py-2 text-xs font-semibold text-red-400 hover:bg-red-900/30 transition-colors"
              >
                <LogOut className="h-4 w-4" />
                Sign Out
              </button>
              
              <button
                onClick={updateProfile}
                className="rounded-lg bg-imessage-blue px-5 py-2 text-xs font-semibold text-white hover:bg-blue-600 transition-colors"
              >
                Save Profile
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
