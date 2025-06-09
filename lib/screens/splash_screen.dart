import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';

class SplashScreen extends StatefulWidget {
  const SplashScreen({super.key});

  @override
  State<SplashScreen> createState() => _SplashScreenState();
}

enum _Phase { animating, loading }

class _SplashScreenState extends State<SplashScreen>
    with TickerProviderStateMixin {
  late final AnimationController _logoController;
  late final Animation<double> _logoAnimation;
  late final AnimationController _textController;
  late final Animation<double> _textAnimation;

  _Phase _phase = _Phase.animating;

  @override
  void initState() {
    super.initState();

    _logoController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1500),
    );
    _logoAnimation =
        CurvedAnimation(parent: _logoController, curve: Curves.elasticOut);

    _textController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1200),
    );
    _textAnimation =
        CurvedAnimation(parent: _textController, curve: Curves.easeIn);

    WidgetsBinding.instance.addPostFrameCallback((_) {
      _logoController.forward().then((_) {
        _textController.forward().then((_) {
          setState(() => _phase = _Phase.loading);
          _handleAutoLogin();
        });
      });
    });
  }

  Future<void> _handleAutoLogin() async {
    final auth = context.read<AuthProvider>();

    final stored = await auth.loadRefreshTokenForDebug();
    if (stored == null) {
      Navigator.pushReplacementNamed(context, '/auth');
      return;
    }

    final success = await auth.tryRefreshToken();
    if (success) {
      final route = auth.routeForCurrentUser();
      Navigator.pushReplacementNamed(context, route);
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
      backgroundColor: const Color(0xFF30d5c8), // Основной цвет приложения
      body: SafeArea(
        child: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              ScaleTransition(
                scale: _logoAnimation,
                child: Container(
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: Colors.white.withOpacity(0.15),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withOpacity(0.1),
                        blurRadius: 12,
                        offset: const Offset(0, 6),
                      ),
                    ],
                  ),
                  padding: const EdgeInsets.all(24),
                  child: const Icon(
                    Icons.local_hospital_rounded,
                    size: 96,
                    color: Colors.white,
                    shadows: [
                      Shadow(
                        color: Colors.black26,
                        offset: Offset(0, 2),
                        blurRadius: 4,
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 28),
              FadeTransition(
                opacity: _textAnimation,
                child: const Text(
                  'Hospital DI',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 36,
                    fontWeight: FontWeight.bold,
                    letterSpacing: 2,
                    shadows: [
                      Shadow(
                        color: Colors.black26,
                        offset: Offset(0, 2),
                        blurRadius: 6,
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 8),
              FadeTransition(
                opacity: _textAnimation,
                child: const Text(
                  'Ваш персональный помощник в здравоохранении',
                  style: TextStyle(
                    color: Colors.white70,
                    fontSize: 16,
                    fontWeight: FontWeight.w500,
                    letterSpacing: 1.1,
                  ),
                ),
              ),
              if (_phase == _Phase.loading) ...[
                const SizedBox(height: 40),
                const CircularProgressIndicator(
                  valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
                  strokeWidth: 3,
                ),
                const SizedBox(height: 12),
                const Text(
                  'Загрузка...',
                  style: TextStyle(color: Colors.white70, fontSize: 14),
                ),
              ]
            ],
          ),
        ),
      ),
    );
  }
}
