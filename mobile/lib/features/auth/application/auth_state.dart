import '../data/auth_models.dart';

/// [AuthInitial] is the very first, synchronous state — shown as a splash
/// screen (see lib/app.dart) while [AuthController] checks whether a stored
/// session is still valid. The router only exists once past this state, so
/// its redirect logic never has to reason about "unknown yet".
sealed class AuthState {
  const AuthState();
}

class AuthInitial extends AuthState {
  const AuthInitial();
}

class AuthUnauthenticated extends AuthState {
  const AuthUnauthenticated();
}

class AuthAuthenticated extends AuthState {
  const AuthAuthenticated(this.user);

  final AuthUser user;
}
