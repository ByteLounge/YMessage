import React, { useState, useEffect, useRef } from 'react';
import { 
  StyleSheet, Text, View, TextInput, TouchableOpacity, 
  FlatList, SafeAreaView, KeyboardAvoidingView, Platform,
  ActivityIndicator, Image, Alert
} from 'react-native';
import { StatusBar } from 'expo-status-bar';

// Define TS Types
interface User {
  id: string;
  username: string;
  display_name: string;
}

interface Chat {
  chat_id: string;
  name: string;
  type: string;
  last_message?: string;
  unread_count: number;
}

interface Message {
  id: string;
  sender_id: string;
  receiver_id?: string;
  content_type: string;
  encrypted_payload: string;
  status: string;
  created_at: string;
}

export default function App() {
  const [currentScreen, setCurrentScreen] = useState<'login' | 'chats' | 'room'>('login');
  const [ipAddress, setIpAddress] = useState('10.0.2.2:8080'); // Android emulator fallback
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // App States
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState('');
  const [chats, setChats] = useState<Chat[]>([]);
  const [activeChat, setActiveChat] = useState<Chat | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputText, setInputText] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  
  const ws = useRef<WebSocket | null>(null);
  const flatListRef = useRef<FlatList | null>(null);

  const getApiUrl = () => `http://${ipAddress}`;
  const getWsUrl = () => `ws://${ipAddress}/api/chat/ws`;

  // Fetch conversations
  const fetchChats = async (authToken: string) => {
    try {
      const response = await fetch(`${getApiUrl()}/api/chat/chats`, {
        headers: { 'Authorization': `Bearer ${authToken}` }
      });
      if (response.ok) {
        const data = await response.json();
        setChats(data);
      }
    } catch (err) {
      console.warn('Failed to fetch chats:', err);
    }
  };

  // Connect WebSockets
  const connectWS = (authToken: string) => {
    try {
      const socket = new WebSocket(`${getWsUrl()}?access_token=${authToken}`);
      ws.current = socket;

      socket.onopen = () => console.log('WS connected on Mobile');
      socket.onmessage = (event) => {
        const data = JSON.parse(event.data);
        if (data.type === 'message') {
          const rawMsg = data.payload as Message;
          
          if (activeChat && (rawMsg.sender_id === activeChat.chat_id || rawMsg.sender_id === user?.id)) {
            setMessages(prev => [...prev, rawMsg]);
          }
          fetchChats(authToken);
        }
      };
      socket.onclose = () => {
        setTimeout(() => connectWS(authToken), 3000);
      };
    } catch (e) {
      console.warn('WS Connect Error:', e);
    }
  };

  const handleLogin = async () => {
    if (!username || !password) {
      setError('Username and Password are required');
      return;
    }
    setError('');
    setLoading(true);

    try {
      const response = await fetch(`${getApiUrl()}/api/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username,
          password,
          platform: 'mobile_rn',
          device_name: 'Expo Go Device',
          identity_key: 'expo_identity_key_placeholder'
        })
      });

      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Login failed');

      setToken(data.access_token);
      setUser(data.user);
      
      // Load chats & WS
      fetchChats(data.access_token);
      connectWS(data.access_token);
      
      setCurrentScreen('chats');
    } catch (err: any) {
      setError(err.message || 'Connection failed. Check backend IP Address.');
    } finally {
      setLoading(false);
    }
  };

  const selectChat = async (selectedChat: Chat) => {
    setActiveChat(selectedChat);
    setCurrentScreen('room');
    setLoading(true);

    try {
      const response = await fetch(`${getApiUrl()}/api/chat/messages?chat_id=${selectedChat.chat_id}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (response.ok) {
        const data = await response.json();
        setMessages(data.messages || []);
      }
    } catch (err) {
      console.warn(err);
    } finally {
      setLoading(false);
    }
  };

  const handleSend = () => {
    if (!inputText.trim() || !activeChat || !ws.current) return;

    const action = {
      type: 'message',
      payload: {
        receiver_id: activeChat.type === 'direct' ? activeChat.chat_id : null,
        group_id: activeChat.type === 'group' ? activeChat.chat_id : null,
        content_type: 'text',
        encrypted_payload: inputText.trim(), // sent as plain-fall for testing or e2ee
        ephemeral_key: 'rn-ephemeral-key',
        counter: 0,
        sender_ratchet_key: 'rn-ratchet-key'
      }
    };

    ws.current.send(JSON.stringify(action));
    
    // Optimistic insert local list
    const localMsg: Message = {
      id: Math.random().toString(),
      sender_id: user?.id || '',
      content_type: 'text',
      encrypted_payload: inputText.trim(),
      status: 'sent',
      created_at: new Date().toISOString()
    };
    setMessages(prev => [...prev, localMsg]);
    setInputText('');
  };

  const handleSearchUser = async () => {
    if (!searchQuery.trim()) return;
    try {
      const response = await fetch(`${getApiUrl()}/api/auth/users/${searchQuery.trim()}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (response.ok) {
        const data = await response.json();
        const newChat: Chat = {
          chat_id: data.id,
          name: data.display_name,
          type: 'direct',
          unread_count: 0
        };
        setChats(prev => [newChat, ...prev]);
        setSearchQuery('');
        selectChat(newChat);
      } else {
        Alert.alert('Not Found', 'No user found with that username');
      }
    } catch (e) {
      Alert.alert('Error', 'Failed to search user');
    }
  };

  // --- RENDERING SCREENS ---

  if (currentScreen === 'login') {
    return (
      <View style={styles.loginContainer}>
        <StatusBar style="light" />
        <View style={styles.logoBadge}>
          <Text style={styles.logoText}>Y</Text>
        </View>
        <Text style={styles.appName}>YMessage</Text>
        <Text style={styles.tagline}>Private. Fast. Beautiful.</Text>

        <View style={styles.card}>
          {error ? <Text style={styles.error}>{error}</Text> : null}

          <Text style={styles.label}>Backend Host IP</Text>
          <TextInput
            value={ipAddress}
            onChangeText={setIpAddress}
            placeholder="192.168.1.XX:8080"
            placeholderTextColor="#52525b"
            style={styles.input}
          />

          <Text style={styles.label}>Username</Text>
          <TextInput
            value={username}
            onChangeText={setUsername}
            placeholder="username"
            placeholderTextColor="#52525b"
            autoCapitalize="none"
            style={styles.input}
          />

          <Text style={styles.label}>Password</Text>
          <TextInput
            value={password}
            onChangeText={setPassword}
            placeholder="••••••••"
            placeholderTextColor="#52525b"
            secureTextEntry
            style={styles.input}
          />

          <TouchableOpacity 
            onPress={handleLogin}
            disabled={loading}
            style={styles.loginButton}
          >
            {loading ? (
              <ActivityIndicator color="#fff" />
            ) : (
              <Text style={styles.loginButtonText}>Sign In</Text>
            )}
          </TouchableOpacity>
        </View>
      </View>
    );
  }

  if (currentScreen === 'chats') {
    return (
      <SafeAreaView style={styles.safeContainer}>
        <StatusBar style="light" />
        <View style={styles.header}>
          <Text style={styles.headerTitle}>Messages</Text>
          <TouchableOpacity 
            onPress={() => {
              setUser(null);
              setToken('');
              setCurrentScreen('login');
            }}
            style={styles.logoutBtn}
          >
            <Text style={styles.logoutText}>Logout</Text>
          </TouchableOpacity>
        </View>

        {/* Search */}
        <View style={styles.searchBar}>
          <TextInput
            placeholder="Search username to start secure chat..."
            placeholderTextColor="#52525b"
            value={searchQuery}
            onChangeText={setSearchQuery}
            onSubmitEditing={handleSearchUser}
            returnKeyType="search"
            style={styles.searchInput}
          />
        </View>

        <FlatList
          data={chats}
          keyExtractor={(item) => item.chat_id}
          renderItem={({ item }) => (
            <TouchableOpacity 
              onPress={() => selectChat(item)}
              style={styles.chatRow}
            >
              <View style={styles.avatar}>
                <Text style={styles.avatarText}>{item.name.charAt(0).toUpperCase()}</Text>
              </View>
              <View style={styles.chatDetails}>
                <Text style={styles.chatName}>{item.name}</Text>
                <Text style={styles.chatPreview} numberOfLines={1}>
                  {item.last_message || 'Tap to open secure chat'}
                </Text>
              </View>
              {item.unread_count > 0 ? (
                <View style={styles.unreadBadge}>
                  <Text style={styles.unreadText}>{item.unread_count}</Text>
                </View>
              ) : null}
            </TouchableOpacity>
          )}
          ListEmptyComponent={
            <View style={styles.emptyContainer}>
              <Text style={styles.emptyText}>No conversations yet</Text>
            </View>
          }
        />
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={styles.safeContainer}>
      <StatusBar style="light" />
      <KeyboardAvoidingView 
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        style={{ flex: 1 }}
      >
        {/* Room Header */}
        <View style={styles.roomHeader}>
          <TouchableOpacity 
            onPress={() => {
              setCurrentScreen('chats');
              fetchChats(token);
            }}
            style={styles.backButton}
          >
            <Text style={styles.backText}>‹ Messages</Text>
          </TouchableOpacity>
          <Text style={styles.roomTitle}>{activeChat?.name}</Text>
          <View style={{ width: 60 }} />
        </View>

        {/* Message logs */}
        <FlatList
          ref={flatListRef}
          data={messages}
          keyExtractor={(item) => item.id}
          onContentSizeChange={() => flatListRef.current?.scrollToEnd({ animated: true })}
          renderItem={({ item }) => {
            const isMe = item.sender_id === user?.id;
            return (
              <View style={[styles.messageBubbleWrapper, isMe ? styles.myMsgWrapper : styles.theirMsgWrapper]}>
                <View style={[styles.bubble, isMe ? styles.myBubble : styles.theirBubble]}>
                  <Text style={styles.bubbleText}>{item.encrypted_payload}</Text>
                </View>
              </View>
            );
          }}
          style={styles.messageList}
        />

        {/* Input Bar */}
        <View style={styles.inputBar}>
          <TextInput
            placeholder="iMessage"
            placeholderTextColor="#71717a"
            value={inputText}
            onChangeText={setInputText}
            style={styles.roomInput}
          />
          <TouchableOpacity 
            onPress={handleSend}
            disabled={!inputText.trim()}
            style={[styles.sendBtn, !inputText.trim() && { opacity: 0.5 }]}
          >
            <Text style={styles.sendText}>▲</Text>
          </TouchableOpacity>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  loginContainer: {
    flex: 1,
    backgroundColor: '#09090b',
    alignItems: 'center',
    justifyContent: 'center',
    padding: 24,
  },
  logoBadge: {
    height: 80,
    width: 80,
    borderRadius: 22,
    backgroundColor: '#007aff',
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: 16
  },
  logoText: {
    color: '#fff',
    fontSize: 44,
    fontWeight: 'bold'
  },
  appName: {
    color: '#fff',
    fontSize: 24,
    fontWeight: '700'
  },
  tagline: {
    color: '#a1a1aa',
    fontSize: 14,
    marginTop: 4,
    marginBottom: 32
  },
  card: {
    width: '100%',
    backgroundColor: '#18181b',
    borderRadius: 20,
    padding: 20,
    borderWidth: 1,
    borderColor: '#27272a'
  },
  label: {
    color: '#a1a1aa',
    fontSize: 11,
    fontWeight: '600',
    textTransform: 'uppercase',
    marginBottom: 6,
    marginTop: 12
  },
  input: {
    backgroundColor: '#09090b',
    borderRadius: 10,
    borderColor: '#27272a',
    borderWidth: 1,
    color: '#fff',
    padding: 12,
    fontSize: 14
  },
  loginButton: {
    backgroundColor: '#007aff',
    borderRadius: 12,
    padding: 14,
    alignItems: 'center',
    justifyContent: 'center',
    marginTop: 24
  },
  loginButtonText: {
    color: '#fff',
    fontSize: 15,
    fontWeight: '600'
  },
  error: {
    color: '#ef4444',
    fontSize: 13,
    textAlign: 'center',
    marginBottom: 12
  },
  safeContainer: {
    flex: 1,
    backgroundColor: '#000',
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'between',
    paddingHorizontal: 16,
    paddingVertical: 12,
    borderBottomWidth: 0.5,
    borderColor: '#1f1f1f'
  },
  headerTitle: {
    color: '#fff',
    fontSize: 24,
    fontWeight: 'bold',
    flex: 1
  },
  logoutBtn: {
    padding: 6
  },
  logoutText: {
    color: '#ef4444',
    fontSize: 14,
    fontWeight: '500'
  },
  searchBar: {
    padding: 10,
  },
  searchInput: {
    backgroundColor: '#1c1c1e',
    borderRadius: 10,
    color: '#fff',
    padding: 8,
    paddingHorizontal: 12,
    fontSize: 14
  },
  chatRow: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 16,
    borderBottomWidth: 0.5,
    borderColor: '#1c1c1e'
  },
  avatar: {
    height: 48,
    width: 48,
    borderRadius: 12,
    backgroundColor: '#27272a',
    alignItems: 'center',
    justifyContent: 'center'
  },
  avatarText: {
    color: '#fff',
    fontSize: 18,
    fontWeight: 'bold'
  },
  chatDetails: {
    flex: 1,
    marginLeft: 14
  },
  chatName: {
    color: '#fff',
    fontSize: 15,
    fontWeight: '600'
  },
  chatPreview: {
    color: '#71717a',
    fontSize: 13,
    marginTop: 3
  },
  unreadBadge: {
    backgroundColor: '#007aff',
    borderRadius: 10,
    paddingHorizontal: 6,
    paddingVertical: 3,
  },
  unreadText: {
    color: '#fff',
    fontSize: 10,
    fontWeight: 'bold'
  },
  emptyContainer: {
    alignItems: 'center',
    justifyContent: 'center',
    padding: 40
  },
  emptyText: {
    color: '#71717a',
    fontSize: 14
  },
  roomHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: 10,
    borderBottomWidth: 0.5,
    borderColor: '#1c1c1e'
  },
  backButton: {
    paddingVertical: 6,
  },
  backText: {
    color: '#007aff',
    fontSize: 16,
  },
  roomTitle: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600'
  },
  messageList: {
    flex: 1,
    padding: 16
  },
  messageBubbleWrapper: {
    flexDirection: 'row',
    marginBottom: 8,
    width: '100%'
  },
  myMsgWrapper: {
    justifyContent: 'flex-end'
  },
  theirMsgWrapper: {
    justifyContent: 'flex-start'
  },
  bubble: {
    borderRadius: 18,
    paddingHorizontal: 14,
    paddingVertical: 8,
    maxWidth: '75%'
  },
  myBubble: {
    backgroundColor: '#007aff',
  },
  theirBubble: {
    backgroundColor: '#262629',
  },
  bubbleText: {
    color: '#fff',
    fontSize: 14,
    lineHeight: 18
  },
  inputBar: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 12,
    borderTopWidth: 0.5,
    borderColor: '#1c1c1e'
  },
  roomInput: {
    flex: 1,
    backgroundColor: '#1c1c1e',
    borderColor: '#262629',
    borderWidth: 1,
    borderRadius: 18,
    color: '#fff',
    paddingHorizontal: 14,
    paddingVertical: 8,
    fontSize: 14
  },
  sendBtn: {
    height: 32,
    width: 32,
    borderRadius: 16,
    backgroundColor: '#007aff',
    alignItems: 'center',
    justifyContent: 'center',
    marginLeft: 8
  },
  sendText: {
    color: '#fff',
    fontSize: 12,
    fontWeight: 'bold'
  }
});
