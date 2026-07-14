import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';
import 'package:ymessage/views/login_view.dart';
import 'package:ymessage/views/chat_view.dart';

void main() {
  runApp(
    MultiProvider(
      providers: [
        ChangeNotifierProvider(create: (_) => AuthProvider()),
        ChangeNotifierProvider(create: (_) => ChatProvider()),
      ],
      child: const YMessageApp(),
    ),
  );
}

class YMessageApp extends StatelessWidget {
  const YMessageApp({Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return CupertinoApp(
      title: 'YMessage',
      theme: const CupertinoThemeData(
        brightness: Brightness.dark,
        primaryColor: CupertinoColors.activeBlue,
        scaffoldBackgroundColor: CupertinoColors.black,
        barBackgroundColor: CupertinoDynamicColor.withBrightness(
          color: Color(0xCC1A1A1A),
          darkColor: Color(0xCC101010),
        ),
      ),
      debugShowCheckedModeBanner: false,
      home: const AuthWrapper(),
    );
  }
}

class AuthWrapper extends StatelessWidget {
  const AuthWrapper({Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    final auth = Provider.of<AuthProvider>(context);
    if (auth.isAuthenticated) {
      return const ChatDashboardView();
    }
    return const LoginView();
  }
}

// Auth State Provider placeholder class (to be fully implemented)
class AuthProvider with ChangeNotifier {
  bool _isAuthenticated = false;
  String _token = '';
  String _userId = '';
  String _username = '';
  String _displayName = '';

  bool get isAuthenticated => _isAuthenticated;
  String get token => _token;
  String get userId => _userId;
  String get displayName => _displayName;

  void setAuthenticated(bool val, String t, String uid, String name, String dName) {
    _isAuthenticated = val;
    _token = t;
    _userId = uid;
    _username = name;
    _displayName = dName;
    notifyListeners();
  }

  void logout() {
    _isAuthenticated = false;
    _token = '';
    _userId = '';
    _username = '';
    _displayName = '';
    notifyListeners();
  }
}

// Chat State Provider placeholder class (to be fully implemented)
class ChatProvider with ChangeNotifier {
  List<dynamic> _chats = [];
  List<dynamic> get chats => _chats;

  void setChats(List<dynamic> newChats) {
    _chats = newChats;
    notifyListeners();
  }
}
