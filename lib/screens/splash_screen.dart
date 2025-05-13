import 'dart:async';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';

class SplashScreen extends StatefulWidget {
  const SplashScreen({super.key});

  @override
  State<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends State<SplashScreen>
    with TickerProviderStateMixin {
  late AnimationController _logoController;
  late Animation<double> _logoAnimation;

  late AnimationController _textController;
  late Animation<double> _textAnimation;

  String? _debugRefreshToken;

  @override
  void initState() {
    super.initState();

    // Инициализируем анимации
    _logoController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1200),
    );
    _logoAnimation =
        CurvedAnimation(parent: _logoController, curve: Curves.easeOutBack);

    _textController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1000),
    );
    _textAnimation =
        CurvedAnimation(parent: _textController, curve: Curves.easeIn);

    _logoController.forward().then((_) => _textController.forward());

    // Читаем токен из secure storage для дебага
    Future.microtask(_loadDebugRefreshToken);

    // По окончании сплеша делаем автологин
    Timer(const Duration(seconds: 3), _handleAutoLogin);
  }

  Future<void> _loadDebugRefreshToken() async {
    final auth = context.read<AuthProvider>();
    final token = await auth.loadRefreshTokenForDebug();
    setState(() {
      _debugRefreshToken = token;
    });
  }

  Future<void> _handleAutoLogin() async {
    final auth = context.read<AuthProvider>();
    final success = await auth.tryRefreshToken();

    if (success) {
      Navigator.pushReplacementNamed(context, '/main');
    } else {
      Navigator.pushReplacementNamed(context, '/auth');
    }
  }

  @override
  void dispose() {
    _logoController.dispose();
    _textController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF6A0DAD),
      body: Stack(
        children: [
          Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                ScaleTransition(
                  scale: _logoAnimation,
                  child: const Icon(
                    Icons.shopping_bag_rounded,
                    size: 100,
                    color: Colors.white,
                  ),
                ),
                const SizedBox(height: 24),
                FadeTransition(
                  opacity: _textAnimation,
                  child: const Text(
                    'Aidyn Market',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 32,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 1.5,
                    ),
                  ),
                ),
              ],
            ),
          ),
          if (_debugRefreshToken != null) // показываем токен, если прочитался
            Positioned(
              bottom: 20,
              left: 20,
              right: 20,
              child: Container(
                padding: const EdgeInsets.all(8),
                color: Colors.white70,
                child: Text(
                  'Debug refreshToken:\n$_debugRefreshToken',
                  style: const TextStyle(
                    color: Colors.black87,
                    fontSize: 12,
                  ),
                  textAlign: TextAlign.center,
                ),
              ),
            ),
        ],
      ),
    );
  }
}
