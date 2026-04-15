from __future__ import annotations

from collections import deque
from dataclasses import dataclass
from threading import Lock

from app.schemas import RouteOutcomeFeedback
from app.services.weighting import clamp, normalize_score


@dataclass(slots=True)
class FeedbackCalibration:
    samples: int = 0
    time_bias: float = 1.0
    risk_bias: float = 0.0
    confidence_bias: float = 0.0


class FeedbackStore:
    def __init__(self, max_samples: int = 500) -> None:
        self._items: deque[RouteOutcomeFeedback] = deque(maxlen=max_samples)
        self._lock = Lock()

    def record(self, feedback: RouteOutcomeFeedback) -> FeedbackCalibration:
        with self._lock:
            self._items.append(feedback)
            return self._calibration_locked()

    def calibration(self) -> FeedbackCalibration:
        with self._lock:
            return self._calibration_locked()

    def _calibration_locked(self) -> FeedbackCalibration:
        if not self._items:
            return FeedbackCalibration()

        total_time_ratio = 0.0
        total_risk_bias = 0.0
        total_confidence_bias = 0.0

        for item in self._items:
            predicted_time = item.predicted_time if item.predicted_time > 0 else item.actual_time
            actual_time = item.actual_time if item.actual_time > 0 else predicted_time
            if predicted_time > 0:
                total_time_ratio += actual_time / predicted_time
            else:
                total_time_ratio += 1.0

            predicted_risk = normalize_score(item.predicted_risk)
            actual_delay = normalize_score(item.actual_delay)
            total_risk_bias += actual_delay - predicted_risk

            delay_error = abs(actual_delay - predicted_risk)
            total_confidence_bias += max(-20.0, 10.0 - delay_error * 100.0)

        samples = len(self._items)
        time_bias = clamp(total_time_ratio / samples, 0.8, 1.4)
        risk_bias = clamp(total_risk_bias / samples, -0.25, 0.25)
        confidence_bias = clamp(total_confidence_bias / samples, -20.0, 20.0)

        return FeedbackCalibration(
            samples=samples,
            time_bias=time_bias,
            risk_bias=risk_bias,
            confidence_bias=confidence_bias,
        )
