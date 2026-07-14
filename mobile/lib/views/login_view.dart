import 'dart:convert';
import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import 'package:ymessage/main.dart';
import 'package:ymessage/services/crypto_service.dart';

class LoginView extends StatefulWidget {
  const LoginView({Key? key}) : super(key: key);

  @override
  State<LoginView> createState() => _LoginViewState();
}

class _LoginViewState extends State<LoginView> {
  final TextEditingController _usernameController = TextEditingController();
  final TextEditingController _emailController = TextEditingController();
  final TextEditingController _passwordController = TextEditingController();
  final TextEditingController _displayNameController = TextEditingController();

  bool _isLogin = true;
  bool _isLoading = false;
  String _errorMessage = '';

  final String _apiBase = 'http://10.0.2.2:8080'; // 10.0.2.2 points to localhost in Android Emulator

  void _handleAuth() async {
    setState(() {
      _isLoading = true;
      _errorMessage = '';
    });

    final cryptoService = CryptoService();
    final prefs = await SharedPreferences.getInstance();

    try {
      if (_isLogin) {
        // 1. Generate/Retrieve client E2EE Identity keys
        String? localPubKey = prefs.getString('ymessage_identity_public_key');
        String? localPrivKey = prefs.getString('ymessage_identity_private_key');

        if (localPubKey == null || localPrivKey == null) {
          final keys = await cryptoService.generateKeyPair();
          localPubKey = await cryptoService.getPublicKeyBase64(keys);
          localPrivKey = await cryptoService.getPrivateKeyBase64(keys);
          await prefs.setString('ymessage_identity_public_key', localPubKey);
          await prefs.setString('ymessage_identity_private_key', localPrivKey);
        }

        // 2. Perform Login Request
        final loginUrl = Uri.parse('$_apiBase/api/auth/login');
        final response = await http.post(
          loginUrl,
          headers: {'Content-Type': 'application/json'},
          body: jsonEncode({
            'username': _usernameController.text.trim(),
            'password': _passwordController.text,
            'platform': 'mobile',
            'device_name': 'Mobile Device',
            'identity_key': localPubKey,
          }),
        );

        final data = jsonDecode(response.body);
        if (response.statusCode != 200) {
          throw Exception(data['error'] ?? 'Login failed');
        }

        final token = data['access_token'];
        final userId = data['user']['id'];
        final username = data['user']['username'];
        final displayName = data['user']['display_name'] ?? username;

        // 3. Generate Signed prekey and One-Time prekeys for E2EE setup
        final spk = await cryptoService.generateKeyPair();
        final spkPub = await cryptoService.getPublicKeyBase64(spk);
        final spkPriv = await cryptoService.getPrivateKeyBase64(spk);
        await prefs.setString('ymessage_signed_public_key', spkPub);
        await prefs.setString('ymessage_signed_private_key', spkPriv);

        final List<Map<String, dynamic>> oneTimeKeysDto = [];
        for (int i = 0; i < 20; i++) {
          final otk = await cryptoService.generateKeyPair();
          final otkPub = await cryptoService.getPublicKeyBase64(otk);
          oneTimeKeysDto.add({
            'key_id': i + 100,
            'key_val': otkPub,
          });
        }

        // 4. Upload prekey bundle to server
        final prekeyUrl = Uri.parse('$_apiBase/api/crypto/prekey');
        await http.post(
          prekeyUrl,
          headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer $token',
          },
          body: jsonEncode({
            'identity_key': localPubKey,
            'signed_prekey': spkPub,
            'signed_prekey_id': 1,
            'signature': 'client_mobile_signature_placeholder',
            'one_time_prekeys': oneTimeKeysDto,
          }),
        );

        // Update auth state in app
        if (mounted) {
          Provider.of<AuthProvider>(context, listen: false).setAuthenticated(
            true,
            token,
            userId,
            username,
            displayName,
          );
        }
      } else {
        // Perform registration
        final registerUrl = Uri.parse('$_apiBase/api/auth/register');
        final response = await http.post(
          registerUrl,
          headers: {'Content-Type': 'application/json'},
          body: jsonEncode({
            'username': _usernameController.text.trim(),
            'email': _emailController.text.trim(),
            'password': _passwordController.text,
            'display_name': _displayNameController.text.trim(),
          }),
        );

        final data = jsonDecode(response.body);
        if (response.statusCode != 201) {
          throw Exception(data['error'] ?? 'Sign up failed');
        }

        setState(() {
          _isLogin = true;
          _errorMessage = 'Account created successfully! Please log in.';
        });
      }
    } catch (e) {
      setState(() {
        _errorMessage = e.toString().replaceAll('Exception:', '').trim();
      });
    } finally {
      if (mounted) {
        setState(() {
          _isLoading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return CupertinoPageScaffold(
      navigationBar: const CupertinoNavigationBar(
        middle: Text('YMessage'),
        backgroundColor: CupertinoColors.black,
      ),
      child: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24.0),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                // Logo
                Center(
                  child: Container(
                    height: 80,
                    width: 80,
                    decoration: BoxDecoration(
                      gradient: const LinearGradient(
                        colors: [CupertinoColors.activeBlue, CupertinoColors.systemTeal],
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                      ),
                      borderRadius: BorderRadius.circular(22),
                    ),
                    child: const Center(
                      child: Text(
                        'Y',
                        style: TextStyle(
                          fontSize: 44,
                          fontWeight: FontWeight.bold,
                          color: CupertinoColors.white,
                        ),
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                const Center(
                  child: Text(
                    'Private. Fast. Beautiful.',
                    style: TextStyle(color: CupertinoColors.systemGrey),
                  ),
                ),
                const SizedBox(height: 32),

                if (_errorMessage.isNotEmpty) ...[
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: _errorMessage.contains('successfully')
                          ? CupertinoColors.systemGreen.withOpacity(0.15)
                          : CupertinoColors.systemRed.withOpacity(0.15),
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(
                        color: _errorMessage.contains('successfully')
                            ? CupertinoColors.systemGreen.withOpacity(0.3)
                            : CupertinoColors.systemRed.withOpacity(0.3),
                      ),
                    ),
                    child: Text(
                      _errorMessage,
                      style: TextStyle(
                        fontSize: 13,
                        color: _errorMessage.contains('successfully')
                            ? CupertinoColors.systemGreen
                            : CupertinoColors.systemRed,
                      ),
                      textAlign: TextAlign.center,
                    ),
                  ),
                  const SizedBox(height: 16),
                ],

                if (!_isLogin) ...[
                  CupertinoTextField(
                    controller: _displayNameController,
                    placeholder: 'Display Name',
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: const Color(0xFF1E1E1E),
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  const SizedBox(height: 12),
                ],

                CupertinoTextField(
                  controller: _usernameController,
                  placeholder: 'Username',
                  padding: const EdgeInsets.all(14),
                  decoration: BoxDecoration(
                    color: const Color(0xFF1E1E1E),
                    borderRadius: BorderRadius.circular(12),
                  ),
                ),
                const SizedBox(height: 12),

                if (!_isLogin) ...[
                  CupertinoTextField(
                    controller: _emailController,
                    placeholder: 'Email',
                    keyboardType: TextInputType.emailAddress,
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: const Color(0xFF1E1E1E),
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  const SizedBox(height: 12),
                ],

                CupertinoTextField(
                  controller: _passwordController,
                  placeholder: 'Password',
                  obscureText: true,
                  padding: const EdgeInsets.all(14),
                  decoration: BoxDecoration(
                    color: const Color(0xFF1E1E1E),
                    borderRadius: BorderRadius.circular(12),
                  ),
                ),
                const SizedBox(height: 20),

                CupertinoButton.filled(
                  onPressed: _isLoading ? null : _handleAuth,
                  child: _isLoading
                      ? const CupertinoActivityIndicator()
                      : Text(_isLogin ? 'Sign In' : 'Sign Up'),
                ),
                const SizedBox(height: 16),

                CupertinoButton(
                  onPressed: () {
                    setState(() {
                      _isLogin = !_isLogin;
                      _errorMessage = '';
                    });
                  },
                  child: Text(
                    _isLogin
                        ? 'Don\'t have an account? Sign Up'
                        : 'Already have an account? Sign In',
                    style: const TextStyle(fontSize: 14),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
