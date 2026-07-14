import 'dart:convert';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart' show Icons;
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:ymessage/main.dart';
import 'package:ymessage/services/crypto_service.dart';
import 'package:http/http.dart' as http;

class ChatDashboardView extends StatefulWidget {
  const ChatDashboardView({Key? key}) : super(key: key);

  @override
  State<ChatDashboardView> createState() => _ChatDashboardViewState();
}

class _ChatDashboardViewState extends State<ChatDashboardView> {
  final TextEditingController _searchController = TextEditingController();
  List<dynamic> _chats = [];
  bool _isLoading = false;
  WebSocketChannel? _channel;

  final String _apiBase = 'http://10.0.2.2:8080';

  @override
  void initState() {
    super.initState();
    final auth = Provider.of<AuthProvider>(context, listen: false);
    _fetchChats(auth.token);
    _initWebSocket(auth.token);
  }

  void _fetchChats(String token) async {
    setState(() => _isLoading = true);
    try {
      final response = await http.get(
        Uri.parse('$_apiBase/api/chat/chats'),
        headers: {'Authorization': 'Bearer $token'},
      );
      if (response.statusCode == 200) {
        setState(() {
          _chats = jsonDecode(response.body);
        });
      }
    } catch (e) {
      print('Error fetching chats: $e');
    } finally {
      setState(() => _isLoading = false);
    }
  }

  void _initWebSocket(String token) {
    final wsUrl = 'ws://10.0.2.2:8080/api/chat/ws?access_token=$token';
    _channel = WebSocketChannel.connect(Uri.parse(wsUrl));

    _channel!.stream.listen((message) {
      final data = jsonDecode(message);
      if (data['type'] == 'message') {
        final auth = Provider.of<AuthProvider>(context, listen: false);
        _fetchChats(auth.token);
      }
    }, onDone: () {
      // Reconnect
      Future.delayed(const Duration(seconds: 3), () {
        if (mounted) _initWebSocket(token);
      });
    });
  }

  @override
  void dispose() {
    _channel?.sink.close();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final auth = Provider.of<AuthProvider>(context);

    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('Messages'),
        trailing: CupertinoButton(
          padding: EdgeInsets.zero,
          child: const Icon(CupertinoIcons.square_pencil, size: 24),
          onPressed: () {
            // Trigger start chat flow
          },
        ),
      ),
      child: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.all(8.0),
              child: CupertinoSearchTextField(
                controller: _searchController,
                placeholder: 'Search Conversations',
              ),
            ),
            Expanded(
              child: _isLoading
                  ? const Center(child: CupertinoActivityIndicator())
                  : ListView.builder(
                      itemCount: _chats.length,
                      itemBuilder: (context, index) {
                        final chat = _chats[index];
                        final unreadCount = chat['unread_count'] ?? 0;

                        return GestureDetector(
                          onTap: () {
                            Navigator.push(
                              context,
                              CupertinoPageRoute(
                                builder: (context) => ChatRoomView(
                                  chatId: chat['chat_id'],
                                  chatName: chat['name'],
                                  chatType: chat['type'],
                                  webSocketChannel: _channel,
                                ),
                              ),
                            ).then((_) => _fetchChats(auth.token));
                          },
                          child: Container(
                            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                            decoration: const BoxDecoration(
                              border: Border(
                                bottom: BorderSide(color: Color(0xFF262626), width: 0.5),
                              ),
                            ),
                            child: Row(
                              children: [
                                Container(
                                  height: 48,
                                  width: 48,
                                  decoration: BoxDecoration(
                                    color: const Color(0xFF2C2C2E),
                                    borderRadius: BorderRadius.circular(12),
                                  ),
                                  child: Center(
                                    child: Text(
                                      chat['name'].substring(0, 1).toUpperCase(),
                                      style: const TextStyle(
                                        fontSize: 20,
                                        fontWeight: FontWeight.bold,
                                        color: CupertinoColors.white,
                                      ),
                                    ),
                                  ),
                                ),
                                const SizedBox(width: 14),
                                Expanded(
                                  child: Column(
                                    crossAxisAlignment: CrossAxisAlignment.start,
                                    children: [
                                      Row(
                                        mainAxisAlignment: MainAxisAlignment.between,
                                        children: [
                                          Text(
                                            chat['name'],
                                            style: const TextStyle(
                                              fontSize: 15,
                                              fontWeight: FontWeight.w600,
                                            ),
                                          ),
                                          Text(
                                            _formatTime(chat['last_active_at']),
                                            style: const TextStyle(
                                              fontSize: 12,
                                              color: CupertinoColors.systemGrey,
                                            ),
                                          ),
                                        ],
                                      ),
                                      const SizedBox(height: 4),
                                      Text(
                                        chat['last_message'] != null
                                            ? (chat['last_message']['encrypted_payload'].length > 30
                                                ? chat['last_message']['encrypted_payload'].substring(0, 30) + '...'
                                                : chat['last_message']['encrypted_payload'])
                                            : 'No messages yet',
                                        style: const TextStyle(
                                          fontSize: 13,
                                          color: CupertinoColors.systemGrey,
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                                if (unreadCount > 0) ...[
                                  const SizedBox(width: 8),
                                  Container(
                                    padding: const EdgeInsets.all(6),
                                    decoration: const BoxDecoration(
                                      color: CupertinoColors.activeBlue,
                                      shape: BoxShape.circle,
                                    ),
                                    child: Text(
                                      unreadCount.toString(),
                                      style: const TextStyle(
                                        fontSize: 10,
                                        color: CupertinoColors.white,
                                        fontWeight: FontWeight.bold,
                                      ),
                                    ),
                                  ),
                                ]
                              ],
                            ),
                          ),
                        );
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }

  String _formatTime(String isoTime) {
    try {
      final dt = DateTime.parse(isoTime);
      return '${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    } catch (_) {
      return '';
    }
  }
}

class ChatRoomView extends StatefulWidget {
  final String chatId;
  final String chatName;
  final String chatType;
  final WebSocketChannel? webSocketChannel;

  const ChatRoomView({
    Key? key,
    required this.chatId,
    required this.chatName,
    required this.chatType,
    this.webSocketChannel,
  }) : super(key: key);

  @override
  State<ChatRoomView> createState() => _ChatRoomViewState();
}

class _ChatRoomViewState extends State<ChatRoomView> {
  final TextEditingController _messageController = TextEditingController();
  final ScrollController _scrollController = ScrollController();
  List<dynamic> _messages = [];
  bool _isLoading = false;
  
  final String _apiBase = 'http://10.0.2.2:8080';

  @override
  void initState() {
    super.initState();
    _loadMessages();
    _listenWS();
  }

  void _listenWS() {
    widget.webSocketChannel?.stream.listen((message) {
      final data = jsonDecode(message);
      if (data['type'] == 'message') {
        _loadMessages();
      }
    });
  }

  void _loadMessages() async {
    final auth = Provider.of<AuthProvider>(context, listen: false);
    setState(() => _isLoading = true);

    try {
      final response = await http.get(
        Uri.parse('$_apiBase/api/chat/messages?chat_id=${widget.chatId}'),
        headers: {'Authorization': 'Bearer ${auth.token}'},
      );
      if (response.statusCode == 200) {
        final data = jsonDecode(response.body);
        final rawMessages = data['messages'] as List<dynamic>;

        // Attempt E2EE decrypting of DMs in Flutter
        final cryptoService = CryptoService();
        final prefs = await SharedPreferences.getInstance();

        // Load shared keys
        String? myPrivKey = prefs.getString('ymessage_identity_private_key');
        
        final List<dynamic> decryptedMessages = [];
        for (var m in rawMessages) {
          String decryptedText = m['encrypted_payload'];
          if (widget.chatType == 'direct' && myPrivKey != null) {
            try {
              // Simulating E2EE decryption in Flutter
              // In production we derive shared secret using partner prekey bundle
              // For demonstration simplicity: check if payload has ':' format
              if (m['encrypted_payload'].contains(':')) {
                // If private key is present, mock decrypt or use service
                decryptedText = '[Encrypted payload decoded]';
              }
            } catch (_) {}
          }
          m['decrypted_text'] = decryptedText;
          decryptedMessages.add(m);
        }

        setState(() {
          _messages = decryptedMessages;
        });

        Future.delayed(const Duration(milliseconds: 100), () {
          if (_scrollController.hasClients) {
            _scrollController.jumpTo(_scrollController.position.maxScrollExtent);
          }
        });
      }
    } catch (e) {
      print(e);
    } finally {
      setState(() => _isLoading = false);
    }
  }

  void _sendMessage() {
    if (_messageController.text.trim().isEmpty) return;
    final auth = Provider.of<AuthProvider>(context, listen: false);

    final payload = {
      'type': 'message',
      'payload': {
        'receiver_id': widget.chatType == 'direct' ? widget.chatId : null,
        'group_id': widget.chatType == 'group' ? widget.chatId : null,
        'content_type': 'text',
        'encrypted_payload': _messageController.text.trim(),
        'ephemeral_key': 'mobile-ephemeral-key',
        'counter': 0,
        'sender_ratchet_key': 'mobile-ratchet-key'
      }
    };

    widget.webSocketChannel?.sink.add(jsonEncode(payload));
    _messageController.clear();
    
    // Optimistic refresh
    Future.delayed(const Duration(milliseconds: 300), _loadMessages);
  }

  @override
  Widget build(BuildContext context) {
    final auth = Provider.of<AuthProvider>(context);

    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: Text(widget.chatName),
        previousPageTitle: 'Messages',
      ),
      child: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: _isLoading && _messages.isEmpty
                  ? const Center(child: CupertinoActivityIndicator())
                  : ListView.builder(
                      controller: _scrollController,
                      padding: const EdgeInsets.all(16),
                      itemCount: _messages.length,
                      itemBuilder: (context, index) {
                        final msg = _messages[index];
                        final isMe = msg['sender_id'] == auth.userId;

                        return Container(
                          margin: const EdgeInsets.only(bottom: 10),
                          alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
                          child: Column(
                            crossAxisAlignment:
                                isMe ? CrossAxisAlignment.end : CrossAxisAlignment.start,
                            children: [
                              Container(
                                padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                                decoration: BoxDecoration(
                                  color: isMe
                                      ? CupertinoColors.activeBlue
                                      : const Color(0xFF262629),
                                  borderRadius: BorderRadius.only(
                                    topLeft: const Radius.circular(16),
                                    topRight: const Radius.circular(16),
                                    bottomLeft: Radius.circular(isMe ? 16 : 2),
                                    bottomRight: Radius.circular(isMe ? 2 : 16),
                                  ),
                                ),
                                child: Text(
                                  msg['decrypted_text'] ?? msg['encrypted_payload'],
                                  style: const TextStyle(
                                    fontSize: 14,
                                    color: CupertinoColors.white,
                                  ),
                                ),
                              ),
                              const SizedBox(height: 2),
                              Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Text(
                                    _formatMsgTime(msg['created_at']),
                                    style: const TextStyle(
                                      fontSize: 10,
                                      color: CupertinoColors.systemGrey,
                                    ),
                                  ),
                                  if (isMe) ...[
                                    const SizedBox(width: 4),
                                    Icon(
                                      msg['status'] == 'read'
                                          ? Icons.done_all
                                          : Icons.done,
                                      size: 11,
                                      color: msg['status'] == 'read'
                                          ? CupertinoColors.activeBlue
                                          : CupertinoColors.systemGrey,
                                    )
                                  ]
                                ],
                              ),
                            ],
                          ),
                        );
                      },
                    ),
            ),
            
            // Bottom Bar
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              decoration: const BoxDecoration(
                border: Border(top: BorderSide(color: Color(0xFF262626), width: 0.5)),
              ),
              child: Row(
                children: [
                  const CupertinoButton(
                    padding: EdgeInsets.zero,
                    child: Icon(CupertinoIcons.paperclip, size: 24),
                    onPressed: null,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: CupertinoTextField(
                      controller: _messageController,
                      placeholder: 'iMessage',
                      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                      decoration: BoxDecoration(
                        color: const Color(0xFF1C1C1E),
                        borderRadius: BorderRadius.circular(18),
                        border: Border.all(color: const Color(0xFF2C2C2E)),
                      ),
                      onSubmitted: (_) => _sendMessage(),
                    ),
                  ),
                  const SizedBox(width: 8),
                  CupertinoButton(
                    padding: EdgeInsets.zero,
                    child: const Icon(CupertinoIcons.arrow_up_circle_fill, size: 28),
                    onPressed: _sendMessage,
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _formatMsgTime(String isoTime) {
    try {
      final dt = DateTime.parse(isoTime);
      return '${dt.hour}:${dt.minute.toString().padLeft(2, '0')}';
    } catch (_) {
      return '';
    }
  }
}
